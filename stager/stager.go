package stager

import (
	"context"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

type StageFunc func(ctx context.Context, runenv *runtime.RunEnv) error

type Stage struct {
	Name string
	Fn   StageFunc
}

type Stager struct {
	client *sync.Client
	runenv *runtime.RunEnv
	stages []*Stage
}

func NewSequential(client *sync.Client, runenv *runtime.RunEnv) *Stager {
	return &Stager{
		client: client,
		runenv: runenv,
	}
}

func (s *Stager) AddStage(name string, fn StageFunc) {
	s.stages = append(s.stages, &Stage{
		Name: name,
		Fn:   fn,
	})
}

func (s *Stager) Run(ctx context.Context) error {
	for _, st := range s.stages {
		st := st

		err := s.client.SignalEvent(ctx, &runtime.Notification{GroupID: s.runenv.TestGroupID, Scope: "stage", EventType: "entry", StageName: st.Name})
		if err != nil {
			return err
		}
		_, err = s.client.SignalAndWait(ctx, sync.State(st.Name+":begin"), s.runenv.TestInstanceCount)
		if err != nil {
			return err
		}

		errc := make(chan error)
		go func() {
			errc <- st.Fn(ctx, s.runenv)
		}()

		select {
		case err := <-errc:
			if err != nil {
				return err
			}
			err = s.client.SignalEvent(ctx, &runtime.Notification{GroupID: s.runenv.TestGroupID, Scope: "stage", EventType: "exit", StageName: st.Name})
			if err != nil {
				return err
			}
			_, err = s.client.SignalAndWait(ctx, sync.State(st.Name+":end"), s.runenv.TestInstanceCount)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
