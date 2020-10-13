package sync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v7"
)

// Barrier sets a barrier on the supplied State that fires when it reaches its
// target value (or higher).
//
// The caller should monitor the channel C returned inside the Barrier object.
// If the barrier is satisfied, the value sent will be nil.
//
// When the context fires, the context's error will be propagated instead. The
// same will occur if the DefaultClient's context fires.
//
// If an internal error occurs,
//
// The returned Barrier object contains a channel (C) that fires when the
// barrier reaches its target, is cancelled, or fails.
//
// The Barrier channel is owned by the DefaultClient, and by no means should the caller
// close it.
// It is safe to use a non-cancellable context here, like the background
// context. No cancellation is needed unless you want to stop the process early.
func (c *DefaultClient) Barrier(ctx context.Context, state State, target int) (*Barrier, error) {
	// a barrier with target zero is satisfied immediately; log a warning as
	// this is probably programmer error.
	if target == 0 {
		c.log.Warnw("requested a barrier with target zero; satisfying immediately", "state", state)
		b := &Barrier{C: make(chan error, 1)}
		b.C <- nil
		close(b.C)
		return b, nil
	}

	rp := c.extractor(ctx)
	if rp == nil {
		return nil, ErrNoRunParameters
	}

	b := &Barrier{
		C:      make(chan error, 1),
		state:  state,
		key:    state.Key(rp),
		target: int64(target),
		ctx:    ctx,
	}

	resultCh := make(chan error)
	c.barrierCh <- &newBarrier{b, resultCh}
	err := <-resultCh
	return b, err
}

// SignalEntry increments the state counter by one, returning the value of the
// new value of the counter, or an error if the operation fails.
func (c *DefaultClient) SignalEntry(ctx context.Context, state State) (after int64, err error) {
	rp := c.extractor(ctx)
	if rp == nil {
		return -1, ErrNoRunParameters
	}

	// Increment a counter on the state key.
	key := state.Key(rp)

	c.log.Debugw("signalling entry to state", "key", key)

	seq, err := c.rclient.Incr(key).Result()
	if err != nil {
		return -1, err
	}

	c.log.Debugw("new value of state", "key", key, "value", seq)
	return seq, err
}

func (c *DefaultClient) SignalEvent(ctx context.Context, event interface{}) (err error) {
	c.log.Debugw("inside signal entry")
	rp := c.extractor(ctx)
	if rp == nil {
		return ErrNoRunParameters
	}

	key := fmt.Sprintf("run:%s:plan:%s:case:%s:run_events", rp.TestRun, rp.TestPlan, rp.TestCase)
	c.log.Debugw("signal entry key", "key", key)

	ev, err := json.Marshal(event)
	if err != nil {
		return err
	}
	c.log.Debugw("signal event marshaled", "key", key)

	args := &redis.XAddArgs{
		Stream: key,
		ID:     "*",
		Values: map[string]interface{}{RedisPayloadKey: ev},
	}

	_, err = c.rclient.XAdd(args).Result()
	if err != nil {
		return err
	}
	c.log.Debugw("signal event xadded", "key", key)

	return nil
}
