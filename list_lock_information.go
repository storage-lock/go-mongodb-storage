package mongodb_storage

import (
	"context"
	"github.com/golang-infrastructure/go-iterator"
	"github.com/storage-lock/go-storage"
	"go.mongodb.org/mongo-driver/mongo"
)

// ListMongoLockIterator 用于迭代列出mongo中的所有的锁
type ListMongoLockIterator struct {
	cursor *mongo.Cursor
}

var _ iterator.Iterator[*storage.LockInformation] = &ListMongoLockIterator{}

func NewListMongoLockIterator(cursor *mongo.Cursor) *ListMongoLockIterator {
	return &ListMongoLockIterator{
		cursor,
	}
}

func (x *ListMongoLockIterator) Next() bool {
	return x.cursor.Next(context.Background())
}

// Value 返回当前游标指向的锁信息。
//
// MongoDB 中锁记录以 MongoLock（bson 字段：_id/owner_id/version/lock_json_string）存储，
// 真正的 LockInformation（包含 LockId/LockCount/ReadLock 等字段）被序列化保存在
// lock_json_string 字段里。之前直接把 bson 文档 Decode 到 storage.LockInformation
// （它只有 json tag，没有 bson tag）会导致除 OwnerId/Version 外的字段全部零值，
// ReadLock/LockCount/LeaseExpireTime 等信息在 List 结果中全部丢失。
// 因此这里先 decode 到 MongoLock，再从 lock_json_string 解析出真正的 LockInformation。
func (x *ListMongoLockIterator) Value() *storage.LockInformation {
	mongoLock := &MongoLock{}
	if err := x.cursor.Decode(mongoLock); err != nil {
		return nil
	}
	info, err := storage.LockInformationFromJsonString(mongoLock.LockJsonString)
	if err != nil {
		return nil
	}
	return info
}
