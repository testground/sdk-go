package peek

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
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

func MonitorEvents(rp *runtime.RunParams, lastid string) (retlastid string, nots []*runtime.Notification) {
	host := "testground-infra-redis-headless"
	port := 6379
	opts := DefaultRedisOpts
	opts.Addr = fmt.Sprintf("%s:%d", host, port)
	client := redis.NewClient(&opts)

	key := fmt.Sprintf("run:%s:plan:%s:case:%s", rp.TestRun, rp.TestPlan, rp.TestCase)
	fmt.Println(key)

	args := new(redis.XReadArgs)
	args.Streams = []string{key, lastid}
	args.Block = 1 * time.Second
	args.Count = 10000

	streams, err := client.XRead(args).Result()
	if err != nil {
		if err == redis.Nil {
			return lastid, nil
		}

		fmt.Println(err.Error())
		return
		//panic(err)
	}

	for _, xr := range streams {
		for _, msg := range xr.Messages {
			payload := msg.Values[sync.RedisPayloadKey].(string)

			notification := &runtime.Notification{}
			err := json.Unmarshal([]byte(payload), notification)
			if err != nil {
				panic(err)
			}

			nots = append(nots, notification)

			retlastid = msg.ID
		}
	}
	return
}
