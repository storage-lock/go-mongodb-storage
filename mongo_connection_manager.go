package mongodb_storage

import (
	"context"
	"github.com/storage-lock/go-storage"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sync"
)

// MongoConnectionManager 负责维护与Mongo数据库的连接
type MongoConnectionManager struct {

	// 连接到数据库的地址
	URI string

	// Mongo客户端
	clientOnce sync.Once
	err        error
	client     *mongo.Client
}

var _ storage.ConnectionManager[*mongo.Client] = &MongoConnectionManager{}

// NewMongoConnectionManagerFromURI 从Mongo uri创建连接管理器
func NewMongoConnectionManagerFromURI(uri string) *MongoConnectionManager {
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
		// 当前还没有创建连接，并且也没有过错误（err主要是用来标记之前连接时发生的错误，有的话就不重复调用了）
		if x.client == nil && x.err == nil {
			// 此处暂不支持定制，如果想修改连接参数的话可以自行创建client从client创建ConnectionManager
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
	if x.client != nil {
		return x.client.Disconnect(ctx)
	}
	return nil
}
