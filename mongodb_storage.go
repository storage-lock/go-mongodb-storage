package mongodb_storage

import (
	"context"
	"errors"
	"github.com/golang-infrastructure/go-iterator"
	ntp_time_provider "github.com/storage-lock/go-ntp-time-provider"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

// ------------------------------------------------ ---------------------------------------------------------------------

// MongoStorage MongoDB的存储引擎实现
type MongoStorage struct {
	options *MongoStorageOptions

	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection

	session mongo.Session

	timeProvider *ntp_time_provider.NTPTimeProvider
}

var _ storage.Storage = &MongoStorage{}

// NewMongoStorage 创建一个基于MongoDB的存储引擎
func NewMongoStorage(ctx context.Context, options *MongoStorageOptions) (*MongoStorage, error) {

	if err := options.Check(); err != nil {
		return nil, err
	}

	s := &MongoStorage{
		options: options,
	}

	err := s.Init(ctx)
	if err != nil {
		return nil, err
	}

	return s, nil
}

const MongoStorageName = "mongodb-storage"

func (x *MongoStorage) GetName() string {
	return MongoStorageName
}

// Capabilities 声明 MongoDB 存储支持的能力
// MongoDB 通过唯一索引 + 条件更新支持 CAS，
// 通过 NTP 时间源提供可靠时间
func (x *MongoStorage) Capabilities() []storage.StorageCapability {
	return []storage.StorageCapability{
		storage.CapabilityCAS,
		storage.CapabilityReliableTime,
	}
}

func (x *MongoStorage) Init(ctx context.Context) error {
	client, err := x.options.ConnectionManager.Take(ctx)
	if err != nil {
		return err
	}
	database := client.Database(x.options.DatabaseName)
	collection := database.Collection(x.options.CollectionName)
	// 初始化
	session, err := client.StartSession()
	if err != nil {
		return err
	}

	x.client = client
	x.session = session
	x.database = database
	x.collection = collection

	// MongoDB没找到好的方式拿服务器上的时间，于是就干脆使用NTP的时间了
	x.timeProvider = ntp_time_provider.NewNTPTimeProvider(nil)

	return nil
}

func (x *MongoStorage) UpdateWithVersion(ctx context.Context, lockId string, exceptedVersion, newVersion storage.Version, lockInformation *storage.LockInformation) error {
	filter := bson.M{
		"_id": bson.M{
			"$eq": lockId,
		},
		"version": bson.M{
			//"$eq": json.Number(strconv.FormatUint(uint64(exceptedVersion), 10)),
			"$eq": exceptedVersion,
		},
	}
	rs, err := x.collection.UpdateOne(ctx, filter, bson.M{
		"$set": &MongoLock{
			ID:             lockId,
			OwnerId:        lockInformation.OwnerId,
			Version:        newVersion,
			LockJsonString: lockInformation.ToJsonString(),
		},
	})
	if err != nil {
		return err
	}
	// 漏洞修复：原用 ModifiedCount==0 判 miss，但 ModifiedCount 仅在文档"实际被修改"时才 +1；
	// 若 filter 匹配但 $set 的值与原值完全相同（理论场景），ModifiedCount 可能为 0 却并非 miss。
	// 改用 MatchedCount（filter 是否匹配到文档）判定，语义更准确。锁每次更新 version 自增，
	// 实际触发"匹配但未修改"概率极低，但用 MatchedCount 是正确判定。
	if rs.MatchedCount == 0 {
		return storage_lock.ErrVersionMiss
	}
	return nil
}

func (x *MongoStorage) CreateWithVersion(ctx context.Context, lockId string, version storage.Version, lockInformation *storage.LockInformation) error {
	_, err := x.collection.InsertOne(ctx, &MongoLock{
		// 锁的ID作为唯一约束，保证同一个锁锁只会存在一个
		ID:             lockId,
		OwnerId:        lockInformation.OwnerId,
		Version:        version,
		LockJsonString: lockInformation.ToJsonString(),
	})
	// 要把ID重复错误转为storage_lock内部的版本miss错误
	if x.isDuplicateKey(err) {
		return storage_lock.ErrVersionMiss
	}
	return err
}

// 判断是否是id重复的错误
// 漏洞修复：原用 strings.Contains(err.Error(), "id dup key") 字符串匹配，依赖 driver 错误消息
// 文本，driver 版本升级或服务端消息变化会误判。改用 driver 标准函数 mongo.IsDuplicateKeyError。
func (x *MongoStorage) isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	return mongo.IsDuplicateKeyError(err)
}

func (x *MongoStorage) DeleteWithVersion(ctx context.Context, lockId string, exceptedVersion storage.Version, lockInformation *storage.LockInformation) error {
	filter := bson.M{
		// 按照锁的ID作为索引删除
		"_id": bson.M{
			"$eq": lockId,
		},
		// 删除的时候保证是自己持有的锁，否则不应该删除成功
		"owner_id": bson.M{
			"$eq": lockInformation.OwnerId,
		},
		// 删除的时候确保版本是匹配的，否则不应该删除成功
		"version": bson.M{
			"$eq": exceptedVersion,
		},
	}
	rs, err := x.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if rs.DeletedCount == 0 {
		// TODO 这里返回的错误是不够准确的，可能还会出现：
		// 1. 锁不存在
		// 2. 锁存在但是不属于这个owner
		return storage_lock.ErrVersionMiss
	}
	return nil
}

func (x *MongoStorage) Get(ctx context.Context, lockId string) (string, error) {
	filter := bson.M{
		"_id": bson.M{
			"$eq": lockId,
		},
	}
	one := x.collection.FindOne(ctx, filter)
	if one.Err() != nil {
		// 把锁不存在的错误统一为接口规定的错误以便上层能够统一处理，上层就不需要关心具体的实现细节了
		if errors.Is(one.Err(), mongo.ErrNoDocuments) {
			return "", storage_lock.ErrLockNotFound
		}
		// 如果是其它类型的错误，就直接返回了
		return "", one.Err()
	}
	mongoLock := &MongoLock{}
	err := one.Decode(mongoLock)
	if err != nil {
		return "", err
	}
	return mongoLock.LockJsonString, nil
}

func (x *MongoStorage) GetTime(ctx context.Context) (time.Time, error) {
	// MongoDB没找到好的办法拿数据库实例的时间，这里就使用一个公共的NTP作为时间源吧
	// TODO
	return x.timeProvider.GetTime(ctx)
	//return time.Now(), nil
}

func (x *MongoStorage) Close(ctx context.Context) error {
	// 漏洞修复：原只把 x.session 置 nil，未调 session.EndSession()，导致 mongo server-side session
	// 资源泄漏（每次 NewMongoStorage 产生一个未关闭的 session）。session 虽未用于事务（CAS 靠单语句
	// 原子性），但仍需显式结束以释放服务端资源。
	if x.session != nil {
		x.session.EndSession(ctx)
		x.session = nil
	}
	// 连接由 ConnectionManager 管理，Storage 只清空引用
	x.collection = nil
	x.database = nil
	x.client = nil
	return nil
}

func (x *MongoStorage) List(ctx context.Context) (iterator.Iterator[*storage.LockInformation], error) {
	cursor, err := x.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	return NewListMongoLockIterator(cursor), nil
}

// ------------------------------------------------ ---------------------------------------------------------------------

// MongoLock 锁在Mongo中存储的结构
type MongoLock struct {

	// 锁的ID，这个字段是一个唯一字段，这个字段会作为Mongo中的Collection的主键字段，保证同一个锁同时只会存在一个
	ID string `bson:"_id"`

	// 锁的当前持有者的ID
	OwnerId string `bson:"owner_id"`

	// 锁的版本，每次修改都会增加1
	Version storage.Version `bson:"version"`

	// 锁的json信息，存储着更上层的通用的锁的信息，这里只需要认为它是一个字符串就可以了
	LockJsonString string `bson:"lock_json_string"`
}

// ------------------------------------------------ ---------------------------------------------------------------------
