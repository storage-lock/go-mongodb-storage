package mongodb_storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type takeErrorManager struct{}

func (takeErrorManager) Name() string { return "take-error" }
func (takeErrorManager) Take(context.Context) (*mongo.Client, error) {
	return nil, errors.New("take failed")
}
func (takeErrorManager) Return(context.Context, *mongo.Client) error { return nil }
func (takeErrorManager) Shutdown(context.Context) error              { return nil }

type fakeSession struct {
	ended bool
}

func (s *fakeSession) EndSession(context.Context) { s.ended = true }

type fixedTimeProvider struct {
	now time.Time
	err error
}

func (p fixedTimeProvider) GetTime(context.Context) (time.Time, error) {
	return p.now, p.err
}

func restoreMongo(t *testing.T) {
	t.Helper()
	connect, disconnect := mongoConnect, mongoDisconnect
	database, collection, start := mongoDatabase, mongoCollection, mongoStartSession
	update, insert, del := mongoUpdateOne, mongoInsertOne, mongoDeleteOne
	findOne, find := mongoFindOne, mongoFind
	t.Cleanup(func() {
		mongoConnect, mongoDisconnect = connect, disconnect
		mongoDatabase, mongoCollection, mongoStartSession = database, collection, start
		mongoUpdateOne, mongoInsertOne, mongoDeleteOne = update, insert, del
		mongoFindOne, mongoFind = findOne, find
	})
}

func newTestMongo(t *testing.T, provider storage.TimeProvider) (*MongoStorage, *fakeSession) {
	t.Helper()
	restoreMongo(t)
	session := &fakeSession{}
	mongoStartSession = func(*mongo.Client) (mongoSession, error) {
		return session, nil
	}
	s, err := NewMongoStorage(context.Background(), &MongoStorageOptions{
		ConnectionManager: NewMongoConnectionManagerFromClient(&mongo.Client{}),
		DatabaseName:      "db",
		CollectionName:    "locks",
		TimeProvider:      provider,
	})
	assert.NoError(t, err)
	return s, session
}

func TestMongoConnectionManager(t *testing.T) {
	restoreMongo(t)
	preset := &mongo.Client{}
	fromClient := NewMongoConnectionManagerFromClient(preset)
	assert.Equal(t, MongoConnectionManagerName, fromClient.Name())
	got, err := fromClient.Take(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, preset, got)
	again, err := fromClient.Take(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, got, again)
	assert.NoError(t, fromClient.Return(context.Background(), got))
	disconnected := false
	mongoDisconnect = func(*mongo.Client, context.Context) error {
		disconnected = true
		return errors.New("disconnect failed")
	}
	var shutErr error
	var shutOK bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		shutErr = fromClient.Shutdown(context.Background())
		shutOK = disconnected
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown blocked")
	}
	assert.EqualError(t, shutErr, "disconnect failed")
	assert.True(t, shutOK)

	empty := &MongoConnectionManager{}
	assert.NoError(t, empty.Shutdown(context.Background()))

	mongoConnect = func(context.Context, string) (*mongo.Client, error) {
		return nil, errors.New("dial failed")
	}
	fromURI := NewMongoConnectionManagerFromURI("mongodb://127.0.0.1:1")
	_, err = fromURI.Take(context.Background())
	assert.EqualError(t, err, "dial failed")
	assert.False(t, fromURI.initialized)

	opened := &mongo.Client{}
	mongoConnect = func(context.Context, string) (*mongo.Client, error) { return opened, nil }
	got, err = fromURI.Take(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, opened, got)
	again, err = fromURI.Take(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, got, again)
}

func TestMongoStorage_Lifecycle(t *testing.T) {
	_, err := NewMongoStorage(context.Background(), NewMongoStorageOptions())
	assert.ErrorIs(t, err, ErrConnectionManagerNil)

	_, err = NewMongoStorage(context.Background(), &MongoStorageOptions{
		ConnectionManager: takeErrorManager{},
		DatabaseName:      "db",
		CollectionName:    "locks",
	})
	assert.EqualError(t, err, "take failed")

	restoreMongo(t)
	mongoStartSession = func(*mongo.Client) (mongoSession, error) {
		return nil, errors.New("session failed")
	}
	_, err = NewMongoStorage(context.Background(), &MongoStorageOptions{
		ConnectionManager: NewMongoConnectionManagerFromClient(&mongo.Client{}),
		DatabaseName:      "db",
		CollectionName:    "locks",
	})
	assert.EqualError(t, err, "session failed")

	s, session := newTestMongo(t, nil)
	assert.NotNil(t, s.timeProvider)
	now := time.Date(2026, 10, 8, 1, 2, 3, 0, time.UTC)
	s.timeProvider = fixedTimeProvider{now: now}
	got, err := s.GetTime(context.Background())
	assert.NoError(t, err)
	assert.True(t, got.Equal(now))
	s.timeProvider = fixedTimeProvider{err: errors.New("clock failed")}
	_, err = s.GetTime(context.Background())
	assert.EqualError(t, err, "clock failed")

	assert.NoError(t, s.Close(context.Background()))
	assert.Nil(t, s.session)
	assert.True(t, session.ended)
	assert.NoError(t, s.Close(context.Background()))
	_, err = (&MongoStorage{}).Watch(context.Background(), "")
	assert.Error(t, err)

	opts := NewMongoStorageOptions().
		SetDatabaseName("db").
		SetCollectionName("locks").
		SetConnectionManager(takeErrorManager{}).
		SetTimeProvider(fixedTimeProvider{now: now})
	assert.Equal(t, "db", opts.DatabaseName)
	assert.Equal(t, "locks", opts.CollectionName)
	assert.NotNil(t, opts.TimeProvider)
	assert.NoError(t, opts.Check())
}

func TestMongoStorage_Mutations(t *testing.T) {
	s, _ := newTestMongo(t, fixedTimeProvider{now: time.Now()})
	info := &storage.LockInformation{OwnerId: "owner"}
	ctx := context.Background()
	duplicate := mongo.WriteException{WriteErrors: mongo.WriteErrors{{Code: 11000, Message: "dup"}}}

	mongoUpdateOne = func(*mongo.Collection, context.Context, any, any) (*mongo.UpdateResult, error) {
		return &mongo.UpdateResult{MatchedCount: 1}, nil
	}
	assert.NoError(t, s.UpdateWithVersion(ctx, "id", 1, 2, info))
	mongoUpdateOne = func(*mongo.Collection, context.Context, any, any) (*mongo.UpdateResult, error) {
		return &mongo.UpdateResult{}, nil
	}
	assert.ErrorIs(t, s.UpdateWithVersion(ctx, "id", 1, 2, info), storage_lock.ErrVersionMiss)
	mongoUpdateOne = func(*mongo.Collection, context.Context, any, any) (*mongo.UpdateResult, error) {
		return nil, errors.New("update failed")
	}
	assert.EqualError(t, s.UpdateWithVersion(ctx, "id", 1, 2, info), "update failed")

	mongoInsertOne = func(*mongo.Collection, context.Context, any) (*mongo.InsertOneResult, error) {
		return &mongo.InsertOneResult{}, nil
	}
	assert.NoError(t, s.CreateWithVersion(ctx, "id", 1, info))
	mongoInsertOne = func(*mongo.Collection, context.Context, any) (*mongo.InsertOneResult, error) {
		return nil, duplicate
	}
	assert.ErrorIs(t, s.CreateWithVersion(ctx, "id", 1, info), storage_lock.ErrVersionMiss)
	mongoInsertOne = func(*mongo.Collection, context.Context, any) (*mongo.InsertOneResult, error) {
		return nil, errors.New("insert failed")
	}
	assert.EqualError(t, s.CreateWithVersion(ctx, "id", 1, info), "insert failed")
	assert.False(t, s.isDuplicateKey(nil))
	assert.True(t, s.isDuplicateKey(duplicate))

	mongoDeleteOne = func(*mongo.Collection, context.Context, any) (*mongo.DeleteResult, error) {
		return &mongo.DeleteResult{DeletedCount: 1}, nil
	}
	assert.NoError(t, s.DeleteWithVersion(ctx, "id", 1, info))
	mongoDeleteOne = func(*mongo.Collection, context.Context, any) (*mongo.DeleteResult, error) {
		return &mongo.DeleteResult{}, nil
	}
	assert.ErrorIs(t, s.DeleteWithVersion(ctx, "id", 1, info), storage_lock.ErrVersionMiss)
	mongoDeleteOne = func(*mongo.Collection, context.Context, any) (*mongo.DeleteResult, error) {
		return nil, errors.New("delete failed")
	}
	assert.EqualError(t, s.DeleteWithVersion(ctx, "id", 1, info), "delete failed")
}

func TestMongoStorage_GetAndList(t *testing.T) {
	s, _ := newTestMongo(t, fixedTimeProvider{now: time.Now()})
	ctx := context.Background()
	jsonText := (&storage.LockInformation{OwnerId: "owner"}).ToJsonString()

	mongoFindOne = func(*mongo.Collection, context.Context, any) *mongo.SingleResult {
		return mongo.NewSingleResultFromDocument(bson.D{}, mongo.ErrNoDocuments, nil)
	}
	_, err := s.Get(ctx, "id")
	assert.ErrorIs(t, err, storage_lock.ErrLockNotFound)

	mongoFindOne = func(*mongo.Collection, context.Context, any) *mongo.SingleResult {
		return mongo.NewSingleResultFromDocument(bson.D{}, errors.New("find failed"), nil)
	}
	_, err = s.Get(ctx, "id")
	assert.EqualError(t, err, "find failed")

	mongoFindOne = func(*mongo.Collection, context.Context, any) *mongo.SingleResult {
		return mongo.NewSingleResultFromDocument(bson.D{{Key: "version", Value: "bad"}}, nil, nil)
	}
	_, err = s.Get(ctx, "id")
	assert.Error(t, err)

	mongoFindOne = func(*mongo.Collection, context.Context, any) *mongo.SingleResult {
		return mongo.NewSingleResultFromDocument(bson.D{{Key: "lock_json_string", Value: jsonText}}, nil, nil)
	}
	got, err := s.Get(ctx, "id")
	assert.NoError(t, err)
	assert.Contains(t, got, "owner")

	mongoFind = func(*mongo.Collection, context.Context, any) (*mongo.Cursor, error) {
		return nil, errors.New("list failed")
	}
	_, err = s.List(ctx)
	assert.EqualError(t, err, "list failed")

	cursor, err := mongo.NewCursorFromDocuments([]any{
		bson.D{{Key: "lock_json_string", Value: 1}},
		bson.D{{Key: "lock_json_string", Value: "not-json"}},
		bson.D{{Key: "lock_json_string", Value: jsonText}},
	}, nil, nil)
	assert.NoError(t, err)
	mongoFind = func(*mongo.Collection, context.Context, any) (*mongo.Cursor, error) {
		return cursor, nil
	}
	listed, err := s.List(ctx)
	assert.NoError(t, err)
	assert.Nil(t, listed.Value())
	count := 0
	for listed.Next() {
		value := listed.Value()
		if value == nil {
			continue
		}
		count++
		assert.Equal(t, "owner", value.OwnerId)
	}
	assert.Equal(t, 1, count)
	assert.False(t, listed.Next())
}

func TestMongoDefaultOperations_Delegate(t *testing.T) {
	_, err := mongoConnect(context.Background(), "://bad")
	assert.Error(t, err)

	client := &mongo.Client{}
	assert.NoError(t, mongoDisconnect(client, context.Background()))
	database := mongoDatabase(client, "db")
	assert.NotNil(t, database)
	collection := mongoCollection(database, "locks")
	assert.NotNil(t, collection)
	_, err = mongoStartSession(client)
	assert.ErrorIs(t, err, mongo.ErrClientDisconnected)

	ctx := context.Background()
	filter := bson.D{{Key: "_id", Value: "id"}}
	_, err = mongoUpdateOne(collection, ctx, filter, bson.D{{Key: "$set", Value: bson.D{{Key: "version", Value: 2}}}})
	assert.Error(t, err)
	_, err = mongoInsertOne(collection, ctx, bson.D{{Key: "_id", Value: "id"}})
	assert.Error(t, err)
	_, err = mongoDeleteOne(collection, ctx, filter)
	assert.Error(t, err)
	assert.Error(t, mongoFindOne(collection, ctx, filter).Err())
	_, err = mongoFind(collection, ctx, bson.D{})
	assert.Error(t, err)
}
