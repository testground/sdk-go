package notification

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

const (
	EnvRedisHost = "REDIS_HOST"
	EnvRedisPort = "REDIS_PORT"
)

var DefaultRedisOpts = redis.Options{
	MinIdleConns:       2,               // allow the pool to downsize to 0 conns.
	PoolSize:           5,               // one for subscriptions, one for nonblocking operations.
	PoolTimeout:        3 * time.Minute, // amount of time a waiter will wait for a conn to become available.
	MaxRetries:         30,
	MinRetryBackoff:    1 * time.Second,
	MaxRetryBackoff:    3 * time.Second,
	DialTimeout:        10 * time.Second,
	ReadTimeout:        10 * time.Second,
	WriteTimeout:       10 * time.Second,
	IdleCheckFrequency: 30 * time.Second,
	MaxConnAge:         2 * time.Minute,
}

var client *redis.Client

func init() {
	var (
		port = 6379
		host = os.Getenv(EnvRedisHost)
	)

	if portStr := os.Getenv(EnvRedisPort); portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil {
			panic(err)
		}
	}

	opts := DefaultRedisOpts
	opts.Addr = fmt.Sprintf("%s:%d", host, port)

	client = redis.NewClient(&opts)
}

func Fetch(rp *runtime.RunParams, lastId string) ([]*runtime.Notification, string, error) {
	key := fmt.Sprintf("run:%s:plan:%s:case:%s", rp.TestRun, rp.TestPlan, rp.TestCase)

	args := new(redis.XReadArgs)
	args.Streams = []string{key, lastId}
	args.Block = 1 * time.Second
	args.Count = 10000

	streams, err := client.XRead(args).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, lastId, nil
		}

		return nil, "", err
	}

	var newId string
	var notifications []*runtime.Notification

	for _, xr := range streams {
		for _, msg := range xr.Messages {
			payload := msg.Values[sync.RedisPayloadKey].(string)

			notification := &runtime.Notification{}
			err := json.Unmarshal([]byte(payload), notification)
			if err != nil {
				panic(err)
			}

			notifications = append(notifications, notification)

			newId = msg.ID
		}
	}

	return notifications, newId, nil
}
