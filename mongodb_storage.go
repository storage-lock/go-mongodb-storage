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

	x.timeProvider = ntp_time_provider.NewNTPTimeProvider(nil)

	return nil
}

func (x *MongoStorage) UpdateWithVersion(ctx context.Context, lockId string, exceptedVersion, newVersion storage.Version, lockInformation *storage.LockInformation) error {
	filter := bson.M{
		"_id": bson.M{
			"$eq": lockId,
		},
		"owner_id": bson.M{
			"$eq": lockInformation.OwnerId,
		},
		"version": bson.M{
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
	if rs.ModifiedCount == 0 {
		// TODO 这里返回的错误是不够准确的，可能还会出现：
		// 1. 锁不存在
		// 2. 锁存在但是不属于这个owner
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
	return err
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

//// IntToBytes 把int转为字节数组
//func IntToBytes(n int) []byte {
//	x := int32(n)
//	bytesBuffer := bytes.NewBuffer([]byte{})
//	binary.Write(bytesBuffer, binary.BigEndian, x)
//	return bytesBuffer.Bytes()
//}

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
	return x.timeProvider.GetTime(ctx)
}

func (x *MongoStorage) Close(ctx context.Context) error {
	if x.client != nil {
		return x.client.Disconnect(ctx)
	}
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
