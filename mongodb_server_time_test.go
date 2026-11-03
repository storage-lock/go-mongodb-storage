package mongodb_storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

// TestGetTimePrefersServerTime 校验 GetTime 优先使用 MongoDB 服务端时间，
// 只有服务端时间不可用时才回退到配置的时间源。
// 这条不变量决定租约判定基准：走 NTP 时各客户端锚定不同基准，租约会被提前判过期。
func TestGetTimePrefersServerTime(t *testing.T) {
	s, _ := newTestMongo(t, fixedTimeProvider{now: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)})
	original := mongoServerTime
	t.Cleanup(func() { mongoServerTime = original })

	serverNow := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	mongoServerTime = func(*mongo.Client, context.Context) (time.Time, error) {
		return serverNow, nil
	}
	got, err := s.GetTime(context.Background())
	if err != nil {
		t.Fatalf("GetTime 报错: %v", err)
	}
	if !got.Equal(serverNow) {
		t.Fatalf("应优先使用服务端时间 %v，实际 %v", serverNow, got)
	}

	// 服务端时间报错时回退到注入的时间源。
	mongoServerTime = func(*mongo.Client, context.Context) (time.Time, error) {
		return time.Time{}, errors.New("hello failed")
	}
	fallback := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	got, err = s.GetTime(context.Background())
	if err != nil {
		t.Fatalf("回退时间源报错: %v", err)
	}
	if !got.Equal(fallback) {
		t.Fatalf("应回退到注入的时间源 %v，实际 %v", fallback, got)
	}

	// 服务端返回零值同样视为不可用，不能把零值当时间写进租约。
	mongoServerTime = func(*mongo.Client, context.Context) (time.Time, error) {
		return time.Time{}, nil
	}
	got, err = s.GetTime(context.Background())
	if err != nil {
		t.Fatalf("零值回退报错: %v", err)
	}
	if !got.Equal(fallback) {
		t.Fatalf("零值应回退到注入的时间源 %v，实际 %v", fallback, got)
	}
}

// TestServerTimeWithoutClient 校验未初始化时不会解引用空客户端。
func TestServerTimeWithoutClient(t *testing.T) {
	if _, err := (&MongoStorage{}).serverTime(context.Background()); !errors.Is(err, ErrMongoNotInit) {
		t.Fatalf("期望 ErrMongoNotInit，实际 %v", err)
	}
}
