package mongodb_storage

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
	"github.com/storage-lock/go-storage"
	"github.com/stretchr/testify/assert"
)

func TestNewMongoStorageOptions(t *testing.T) {
	opts := NewMongoStorageOptions()
	assert.NotNil(t, opts)
	assert.Equal(t, storage.DefaultStorageDatabaseName, opts.DatabaseName)
	assert.Equal(t, storage.DefaultStorageTableName, opts.CollectionName)
	assert.Nil(t, opts.ConnectionManager)
}

func TestNewMongoStorageOptionsWithURI(t *testing.T) {
	opts := NewMongoStorageOptionsWithURI("mongodb://localhost:27017")
	assert.NotNil(t, opts)
	assert.NotNil(t, opts.ConnectionManager)
}

func TestMongoStorageOptions_SetDatabaseName(t *testing.T) {
	opts := NewMongoStorageOptions()
	assert.NotNil(t, opts)
	opts.SetDatabaseName("custom_db")
	assert.Equal(t, "custom_db", opts.DatabaseName)
}

func TestMongoStorageOptions_SetCollectionName(t *testing.T) {
	opts := NewMongoStorageOptions()
	assert.NotNil(t, opts)
	opts.SetCollectionName("custom_collection")
	assert.Equal(t, "custom_collection", opts.CollectionName)
}

func TestMongoStorageOptions_Check_EmptyDatabaseName(t *testing.T) {
	opts := NewMongoStorageOptions()
	assert.NotNil(t, opts)
	opts.DatabaseName = ""
	err := opts.Check()
	assert.Error(t, err)
	assert.Equal(t, ErrDatabaseNameEmpty, err)
}

func TestMongoStorageOptions_Check_EmptyCollectionName(t *testing.T) {
	opts := NewMongoStorageOptions()
	assert.NotNil(t, opts)
	opts.CollectionName = ""
	err := opts.Check()
	assert.Error(t, err)
	assert.Equal(t, ErrCollectionNameEmpty, err)
}

func TestMongoStorageOptions_Check_NilConnectionManager(t *testing.T) {
	opts := NewMongoStorageOptions()
	assert.NotNil(t, opts)
	opts.ConnectionManager = nil
	err := opts.Check()
	assert.Error(t, err)
	assert.Equal(t, ErrConnectionManagerNil, err)
}

func TestMongoStorageOptions_Check_ValidOptions(t *testing.T) {
	opts := NewMongoStorageOptions()
	assert.NotNil(t, opts)
	opts.ConnectionManager = &testMongoConnectionManager{}
	err := opts.Check()
	assert.NoError(t, err)
}

func TestNewMongoStorage_NilOptions(t *testing.T) {
	assert.Panics(t, func() {
		NewMongoStorage(context.Background(), nil)
	})
}

func TestMongoStorage_GetName(t *testing.T) {
	s := &MongoStorage{}
	name := s.GetName()
	assert.Equal(t, MongoStorageName, name)
}

func TestMongoStorage_Capabilities(t *testing.T) {
	s := &MongoStorage{}
	caps := s.Capabilities()
	assert.NotNil(t, caps)
	assert.Len(t, caps, 3)
	assert.Equal(t, storage.CapabilityCAS, caps[0])
	assert.Equal(t, storage.CapabilityReliableTime, caps[1])
	assert.Equal(t, storage.CapabilityWatch, caps[2])
}

type testMongoConnectionManager struct{}

func (t *testMongoConnectionManager) Name() string {
	return "test-mongo-connection-manager"
}

func (t *testMongoConnectionManager) Take(ctx context.Context) (*mongo.Client, error) {
	return nil, nil
}

func (t *testMongoConnectionManager) Return(ctx context.Context, client *mongo.Client) error {
	return nil
}

func (t *testMongoConnectionManager) Shutdown(ctx context.Context) error {
	return nil
}
