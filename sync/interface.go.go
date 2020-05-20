package sync

import (
	"context"
	"io"
)

// Interface is used by Testground components to enable mocking/stubbing in
// tests. Consequently, it doesn't have the Must* variants, as those are useful
// for test plans only.
//
// If you're a test plan writer, you should not be using this.
type Interface interface {
	io.Closer

	Publish(ctx context.Context, topic *Topic, payload interface{}) (seq int64, err error)
	Subscribe(ctx context.Context, topic *Topic, ch interface{}) (*Subscription, error)
	PublishAndWait(ctx context.Context, topic *Topic, payload interface{}, state State, target int) (seq int64, err error)
	PublishSubscribe(ctx context.Context, topic *Topic, payload interface{}, ch interface{}) (seq int64, sub *Subscription, err error)

	Barrier(ctx context.Context, state State, target int) (*Barrier, error)
	SignalEntry(ctx context.Context, state State) (after int64, err error)
	SignalAndWait(ctx context.Context, state State, target int) (seq int64, err error)
}
