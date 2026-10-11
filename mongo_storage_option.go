package mongodb_storage

import (
	"github.com/storage-lock/go-storage"
	"go.mongodb.org/mongo-driver/mongo"
)

// MongoStorageOptions Mongo的存储选项
type MongoStorageOptions struct {

	// 获取连接
	ConnectionManager storage.ConnectionManager[*mongo.Client]

	// 要存储到的数据库的名称
	DatabaseName string

	// 集合名称
	CollectionName string

	// TimeProvider 可选，自定义时间源。MongoDB 本身没有便捷的方式拿到服务器时间，
	// 默认使用公共 NTP（见 GetTime），但公共 NTP 依赖外网可达性，延迟也不稳定；
	// 内网部署或测试场景可以通过这里注入自定义实现（比如指向内网 NTP，或直接用
	// 本地时钟）来避免这个依赖。为空则回退到默认的 NTP 实现。
	TimeProvider storage.TimeProvider
}

func NewMongoStorageOptions() *MongoStorageOptions {
	return &MongoStorageOptions{
		DatabaseName:   storage.DefaultStorageDatabaseName,
		CollectionName: storage.DefaultStorageTableName,
	}
}

func NewMongoStorageOptionsWithURI(uri string) *MongoStorageOptions {
	return NewMongoStorageOptions().SetConnectionManager(NewMongoConnectionManagerFromURI(uri))
}

func (x *MongoStorageOptions) SetDatabaseName(databaseName string) *MongoStorageOptions {
	x.DatabaseName = databaseName
	return x
}

func (x *MongoStorageOptions) SetCollectionName(collectionName string) *MongoStorageOptions {
	x.CollectionName = collectionName
	return x
}

func (x *MongoStorageOptions) SetConnectionManager(connectionProvider storage.ConnectionManager[*mongo.Client]) *MongoStorageOptions {
	x.ConnectionManager = connectionProvider
	return x
}

func (x *MongoStorageOptions) SetTimeProvider(timeProvider storage.TimeProvider) *MongoStorageOptions {
	x.TimeProvider = timeProvider
	return x
}

func (x *MongoStorageOptions) Check() error {
	if x.DatabaseName == "" {
		return ErrDatabaseNameEmpty
	}
	if x.CollectionName == "" {
		return ErrCollectionNameEmpty
	}
	if x.ConnectionManager == nil {
		return ErrConnectionManagerNil
	}
	return nil
}
