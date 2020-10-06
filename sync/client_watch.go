package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/testground/sdk-go/runtime"
	"go.uber.org/zap"
)

// WatchClient is used by the Testground daemon to monitor all emitted events by the testplans,
// in particular the terminal events, such as SuccessEvent, FailureEvent and CrashEvent.
type WatchClient struct {
	ctx     context.Context
	rclient *redis.Client
	log     *zap.SugaredLogger
}

func NewWatchClient(ctx context.Context, log *zap.SugaredLogger) (*WatchClient, error) {
	rclient, err := redisClient(ctx, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}

	return &WatchClient{
		ctx,
		rclient,
		log,
	}, nil
}

func (w *WatchClient) FetchAllEvents(rp *runtime.RunParams) ([]*runtime.Event, error) {
	key := fmt.Sprintf("run:%s:plan:%s:case:%s", rp.TestRun, rp.TestPlan, rp.TestCase)

	var events []*runtime.Event

	id := "0"
	for {
		args := &redis.XReadArgs{}
		args.Streams = []string{key, id}
		args.Block = 1 * time.Second
		args.Count = 10000

		streams, err := w.rclient.XRead(args).Result()
		if err != nil {
			if err == redis.Nil {
				break
			}

			return nil, err
		}

		newId := id
		for _, xr := range streams {
			for _, msg := range xr.Messages {
				payload := msg.Values[RedisPayloadKey].(string)

				ev := &runtime.Event{}
				err := json.Unmarshal([]byte(payload), ev)
				if err != nil {
					panic(err)
				}

				events = append(events, ev)

				newId = msg.ID
			}
		}

		// exit if the cursor hasn't changed
		if newId == id {
			break
		}
		id = newId
	}

	return events, nil
}
