package mongodb_storage

import "fmt"

var (

	// ErrDatabaseNameEmpty 参数中数据库名字为空
	ErrDatabaseNameEmpty = fmt.Errorf("DatabaseName can not empty")

	// ErrCollectionNameEmpty 参数中集合名字为空
	ErrCollectionNameEmpty = fmt.Errorf("CollectionName can not empty")

	// ErrConnectionManagerNil 连接管理器没有指定
	ErrConnectionManagerNil = fmt.Errorf("ConnectionManager can not nil")
)
