package sync

import (
	"context"
	"io"

	"github.com/go-redis/redis/v7"
)

type Client interface {
	io.Closer

	Publish(ctx context.Context, topic *Topic, payload interface{}) (seq int64, err error)
	Subscribe(ctx context.Context, topic *Topic, ch interface{}) (*Subscription, error)
	PublishAndWait(ctx context.Context, topic *Topic, payload interface{}, state State, target int) (seq int64, err error)
	PublishSubscribe(ctx context.Context, topic *Topic, payload interface{}, ch interface{}) (seq int64, sub *Subscription, err error)

	Barrier(ctx context.Context, state State, target int) (*Barrier, error)
	SignalEntry(ctx context.Context, state State) (after int64, err error)
	SignalAndWait(ctx context.Context, state State, target int) (seq int64, err error)

	MustBarrier(ctx context.Context, state State, target int) *Barrier
	MustSignalEntry(ctx context.Context, state State) int64
	MustSubscribe(ctx context.Context, topic *Topic, ch interface{}) *Subscription
	MustPublish(ctx context.Context, topic *Topic, payload interface{}) (seq int64)

	MustPublishAndWait(ctx context.Context, topic *Topic, payload interface{}, state State, target int) (seq int64)
	MustPublishSubscribe(ctx context.Context, topic *Topic, payload interface{}, ch interface{}) (seq int64, sub *Subscription)
	MustSignalAndWait(ctx context.Context, state State, target int) (seq int64)

	// RedisClient returns the Redis client that underpins sync.DefaultClient.
	//
	// USE WITH CAUTION.
	//
	// Redis is a shared-memory environment, and use of RedisClient() comes with all the
	// usual multithreding caveats.  Use of this method is discouraged where high-level
	// primitives in the sync package suffice to accomplish the task at hand.
	RedisClient() *redis.Client
}
