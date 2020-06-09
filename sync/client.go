package sync

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/testground/sdk-go/runtime"

	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
)

const (
	RedisPayloadKey = "p"

	EnvRedisHost = "REDIS_HOST"
	EnvRedisPort = "REDIS_PORT"
)

// ErrNoRunParameters is returned by the generic client when an unbound context
// is passed in. See WithRunParams to bind RunParams to the context.
var ErrNoRunParameters = fmt.Errorf("no run parameters provided")

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

type Client struct {
	*sugarOperations

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	log       *zap.SugaredLogger
	extractor func(ctx context.Context) (rp *runtime.RunParams)

	rclient *redis.Client

	barrierCh chan *newBarrier
	newSubCh  chan *newSubscription
}

// NewBoundClient returns a new sync Client that is bound to the provided
// RunEnv. All operations will be automatically scoped to the keyspace of that
// run.
//
// The context passed in here will govern the lifecycle of the client.
// Cancelling it will cancel all ongoing operations. However, for a clean
// closure, the user should call Close().
//
// For test plans, a suitable context to pass here is the background context.
func NewBoundClient(ctx context.Context, runenv *runtime.RunEnv) (*Client, error) {
	log := runenv.SLogger()

	rclient, err := redisClient(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	return newClient(ctx, rclient, log, func(ctx context.Context) *runtime.RunParams {
		return &runenv.RunParams
	})
}

// MustBoundClient creates a new bound client by calling NewBoundClient, and
// panicking if it errors.
func MustBoundClient(ctx context.Context, runenv *runtime.RunEnv) *Client {
	c, err := NewBoundClient(ctx, runenv)
	if err != nil {
		panic(err)
	}
	return c
}

// NewGenericClient returns a new sync Client that is bound to no RunEnv.
// It is intended to be used by testground services like the sidecar.
//
// All operations expect to find the RunParams of the run to scope its actions
// inside the supplied context.Context. Call WithRunParams to bind the
// appropriate RunParams.
//
// The context passed in here will govern the lifecycle of the client.
// Cancelling it will cancel all ongoing operations. However, for a clean
// closure, the user should call Close().
//
// A suitable context to pass here is the background context of the main
// process.
func NewGenericClient(ctx context.Context, log *zap.SugaredLogger) (*Client, error) {
	rclient, err := redisClient(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	return newClient(ctx, rclient, log, GetRunParams)
}

func NewGenericClientWithRedis(ctx context.Context, rclient *redis.Client, log *zap.SugaredLogger) (*Client, error) {
	return newClient(ctx, rclient, log, GetRunParams)
}

// MustGenericClient creates a new generic client by calling NewGenericClient,
// and panicking if it errors.
func MustGenericClient(ctx context.Context, log *zap.SugaredLogger) *Client {
	c, err := NewGenericClient(ctx, log)
	if err != nil {
		panic(err)
	}
	return c
}

// newClient creates a new sync client.
func newClient(ctx context.Context, redisClient *redis.Client, log *zap.SugaredLogger, extractor func(ctx context.Context) *runtime.RunParams) (*Client, error) {
	ctx, cancel := context.WithCancel(ctx)
	c := &Client{
		ctx:       ctx,
		cancel:    cancel,
		log:       log,
		extractor: extractor,
		rclient:   redisClient,
		barrierCh: make(chan *newBarrier),
		newSubCh:  make(chan *newSubscription),
	}

	c.sugarOperations = &sugarOperations{c}

	c.wg.Add(2)
	go c.barrierWorker()
	go c.subscriptionWorker()

	if debug := log.Desugar().Core().Enabled(zap.DebugLevel); debug {
		go func() {
			tick := time.NewTicker(1 * time.Second)
			defer tick.Stop()

			for {
				select {
				case <-tick.C:
					stats := redisClient.PoolStats()
					log.Debugw("redis pool stats", "stats", stats)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return c, nil
}

// Close closes this client, cancels ongoing operations, and releases resources.
func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()

	return c.rclient.Close()
}

// newSubscription is an ancillary type used when creating a new Subscription.
type newSubscription struct {
	sub      *Subscription
	resultCh chan error
}

// newBarrier is an ancillary type used when creating a new Barrier.
type newBarrier struct {
	barrier  *Barrier
	resultCh chan error
}

// redisClient returns a Redis client constructed from this process' environment
// variables.
func redisClient(ctx context.Context, log *zap.SugaredLogger) (client *redis.Client, err error) {
	var (
		port = 6379
		host = os.Getenv(EnvRedisHost)
	)

	if portStr := os.Getenv(EnvRedisPort); portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse port '%q': %w", portStr, err)
		}
	}

	log.Debugw("trying redis host", "host", host, "port", port)

	opts := DefaultRedisOpts
	opts.Addr = fmt.Sprintf("%s:%d", host, port)
	client = redis.NewClient(&opts).WithContext(ctx)

	if err := client.Ping().Err(); err != nil {
		_ = client.Close()
		log.Errorw("failed to ping redis host", "host", host, "port", port, "error", err)
		return nil, err
	}

	log.Debugw("redis ping OK", "opts", opts)

	return client, nil
}
