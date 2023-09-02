package mongodb_storage

import (
	"context"
	storage_test_helper "github.com/storage-lock/go-storage-test-helper"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestNewMongoStorage(t *testing.T) {
	envName := "STORAGE_LOCK_MONGO_URI"
	uri := os.Getenv(envName)
	assert.NotEmpty(t, uri)
	connectionGetter := NewMongoConnectionManagerFromURI(uri)
	s, err := NewMongoStorage(context.Background(), &MongoStorageOptions{
		ConnectionManager: connectionGetter,
		DatabaseName:      storage_test_helper.TestDatabaseName,
		CollectionName:    storage_test_helper.TestTableName,
	})
	assert.Nil(t, err)
	storage_test_helper.TestStorage(t, s)
}
