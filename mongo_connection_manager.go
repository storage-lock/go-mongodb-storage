package mongodb_storage

import (
	"context"
	"github.com/storage-lock/go-storage"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sync"
)

// MongoConnectionManager 根据URI连接Mongo服务器获取连接
type MongoConnectionManager struct {

	// 连接到数据库的选项
	URI string

	// Mongo客户端
	clientOnce sync.Once
	err        error
	client     *mongo.Client
}

var _ storage.ConnectionManager[*mongo.Client] = &MongoConnectionManager{}

// NewMongoConnectionManager 从Mongo uri创建连接管理器
func NewMongoConnectionManager(uri string) *MongoConnectionManager {
	return &MongoConnectionManager{
		URI: uri,
	}
}

// NewMongoConnectionManagerFromClient 复用已经存在的mongo client，从其创建连接管理器
func NewMongoConnectionManagerFromClient(client *mongo.Client) *MongoConnectionManager {
	return &MongoConnectionManager{
		client: client,
	}
}

const MongoConnectionManagerName = "mongodb-connection-manager"

func (x *MongoConnectionManager) Name() string {
	return MongoConnectionManagerName
}

func (x *MongoConnectionManager) Take(ctx context.Context) (*mongo.Client, error) {
	x.clientOnce.Do(func() {
		if x.client == nil {
			client, err := mongo.Connect(ctx, options.Client().ApplyURI(x.URI))
			if err != nil {
				x.err = err
				return
			}
			x.client = client
		}
	})
	return x.client, x.err
}

func (x *MongoConnectionManager) Return(ctx context.Context, connection *mongo.Client) error {
	return nil
}

func (x *MongoConnectionManager) Shutdown(ctx context.Context) error {
	// 连接池关闭的时候断掉连接，即使它是被复用的
	return x.client.Disconnect(ctx)
}
