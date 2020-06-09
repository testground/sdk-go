package peek

import (
	"fmt"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
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

func NetworkInitialisedBarrier(rp *runtime.RunParams) {
	host := "testground-infra-redis-headless"
	port := 6379
	opts := DefaultRedisOpts
	opts.Addr = fmt.Sprintf("%s:%d", host, port)
	client := redis.NewClient(&opts)

	//ctx := context.Background()
	//syncclient, err := sync.NewGenericClientWithRedis(ctx, client, logging.S())
	//if err != nil {
	//panic(err)
	//}

	//func (c *Client) Subscribe(ctx context.Context, topic *Topic, ch interface{}) (*Subscription, error) {
	//syncclient.Subscribe()

	state := sync.State("network-initialized")

	key := state.Key(rp)

	vals, err := client.MGet(key).Result()
	if err != nil {
		panic(err)
	}

	v := vals[0]

	if v == nil {
		return
	}

	curr, err := strconv.ParseInt(v.(string), 10, 64)
	if err != nil {
		panic(err)
	}

	fmt.Println(curr)
	spew.Dump(curr)
}
