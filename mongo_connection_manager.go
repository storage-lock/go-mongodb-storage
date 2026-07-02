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

	// initMu 保护下面的初始化状态。原用 sync.Once，Connect 首次失败后 err 永久缓存，
	// 后续 Take 永远返回同一 err 无法重试（漏洞4/liveness，不破互斥性——拿不到 client
	// 则 CAS 无法发起）。改为 mutex + initialized 标志：成功后缓存 client 不再重复 Connect；
	// 失败不缓存 err，下次 Take 可重试。
	initMu      sync.Mutex
	initialized bool
	client      *mongo.Client
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
		client:      client,
		initialized: true, // 外部预设 client，Take 不再 Connect
	}
}

const MongoConnectionManagerName = "mongodb-connection-manager"

func (x *MongoConnectionManager) Name() string {
	return MongoConnectionManagerName
}

// Take 获取 Mongo 客户端。
//
// 漏洞4 修复：原 sync.Once 在 mongo.Connect 失败时永久缓存 err，无法重试。
// mongo.Connect 可能因网络/DNS 问题失败，永久缓存对临时性故障不友好（liveness）。
// 改为 mutex + initialized：成功后缓存 client 不重复 Connect；失败不缓存 err 可重试。
func (x *MongoConnectionManager) Take(ctx context.Context) (*mongo.Client, error) {
	x.initMu.Lock()
	defer x.initMu.Unlock()
	if x.initialized {
		return x.client, nil
	}
	// 此处暂不支持定制，如果想修改连接参数的话可以自行创建client从client创建ConnectionManager
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(x.URI))
	if err != nil {
		// 失败不缓存，下次 Take 可重试
		return nil, err
	}
	x.client = client
	x.initialized = true
	return x.client, nil
}

func (x *MongoConnectionManager) Return(ctx context.Context, connection *mongo.Client) error {
	return nil
}

func (x *MongoConnectionManager) Shutdown(ctx context.Context) error {
	// 连接池关闭的时候断掉连接，即使它是被复用的
	x.initMu.Lock()
	client := x.client
	x.initMu.Unlock()
	if client != nil {
		return client.Disconnect(ctx)
	}
	return nil
}
