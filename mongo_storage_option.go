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
}

func NewMongoStorageOptions() *MongoStorageOptions {
	return &MongoStorageOptions{
		DatabaseName:   storage.DefaultStorageDatabaseName,
		CollectionName: storage.DefaultStorageTableName,
	}
}

func NewMongoStorageOptionsWithURI(uri string) *MongoStorageOptions {
	return NewMongoStorageOptions().SetConnectionManager(NewMongoConnectionManager(uri))
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
