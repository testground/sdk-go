package sync

import (
	"context"
	"io"
)

type Interface interface {
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
}
