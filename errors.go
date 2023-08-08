package mongodb_storage

import "fmt"

var (
	ErrDatabaseNameEmpty    = fmt.Errorf("DatabaseName can not empty")
	ErrCollectionNameEmpty  = fmt.Errorf("CollectionName can not empty")
	ErrConnectionManagerNil = fmt.Errorf("ConnectionManager can not nil")
)
