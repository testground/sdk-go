package sync

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
)

func TestRedisHost(t *testing.T) {
	realRedisHost := os.Getenv(EnvRedisHost)
	defer os.Setenv(EnvRedisHost, realRedisHost)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = os.Setenv(EnvRedisHost, "redis-does-not-exist.example.com")
	client, err := redisClient(ctx, zap.S())
	if err == nil {
		_ = client.Close()
		t.Error("should not have found redis host")
	}

	_ = os.Setenv(EnvRedisHost, "redis-does-not-exist.example.com")
	client, err = redisClient(ctx, zap.S())
	if err == nil {
		_ = client.Close()
		t.Error("should not have found redis host")
	}

	realHost := realRedisHost
	if realHost == "" {
		realHost = "localhost"
	}
	_ = os.Setenv(EnvRedisHost, realHost)
	client, err = redisClient(ctx, zap.S())
	if err != nil {
		t.Errorf("expected to establish connection to redis, but failed with: %s", err)
	}
	_ = client.Close()
}

func TestConnUnblock(t *testing.T) {
	client := redis.NewClient(&redis.Options{})
	c := client.Conn()
	id, _ := c.ClientID().Result()

	ch := make(chan struct{})
	go func() {
		timer := time.AfterFunc(1*time.Second, func() { close(ch) })
		_, _ = c.XRead(&redis.XReadArgs{Streams: []string{"aaaa", "0"}, Block: 0}).Result()
		timer.Stop()
	}()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("XREAD unexpectedly returned early")
	}

	unblocked, err := client.ClientUnblock(id).Result()
	if err != nil {
		t.Fatal(err)
	}
	if unblocked != 1 {
		t.Fatal("expected CLIENT UNBLOCK to return 1")
	}
	for i := 0; i < 10; i++ {
		id2, err := c.ClientID().Result()
		if err != nil {
			t.Fatal(err)
		}
		if id != id2 {
			t.Errorf("expected client id to be: %d, was: %d", id, id2)
		}
	}
}
