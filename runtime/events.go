package runtime

import (
	"fmt"
	"runtime/debug"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	EventType    string
	EventOutcome string
)

const (
	EventTypeStart   = EventType("start")
	EventTypeMessage = EventType("message")
	EventTypeFinish  = EventType("finish")

	EventOutcomeOK      = EventOutcome("ok")
	EventOutcomeFailed  = EventOutcome("failed")
	EventOutcomeCrashed = EventOutcome("crashed")
)

type Event struct {
	Type       EventType    `json:"type"`
	Outcome    EventOutcome `json:"outcome,omitempty"`
	Error      string       `json:"error,omitempty"`
	Stacktrace string       `json:"stacktrace,omitempty"`
	Message    string       `json:"message,omitempty"`
	Runenv     *RunParams   `json:"runenv,omitempty"`
}

func (e Event) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("type", string(e.Type))

	if e.Outcome != "" {
		oe.AddString("outcome", string(e.Outcome))
	}
	if e.Error != "" {
		oe.AddString("error", e.Error)
	}
	if e.Stacktrace != "" {
		oe.AddString("stacktrace", e.Stacktrace)
	}
	if e.Message != "" {
		oe.AddString("message", e.Message)
	}
	if e.Runenv != nil {
		if err := oe.AddObject("runenv", e.Runenv); err != nil {
			return err
		}
	}

	return nil
}

func (rp *RunParams) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("plan", rp.TestPlan)
	oe.AddString("case", rp.TestCase)
	if err := oe.AddReflected("params", rp.TestInstanceParams); err != nil {
		return err
	}
	oe.AddInt("instances", rp.TestInstanceCount)
	oe.AddString("outputs_path", rp.TestOutputsPath)
	oe.AddString("network", func() string {
		if rp.TestSubnet == nil {
			return ""
		}
		return rp.TestSubnet.Network()
	}())

	oe.AddString("group", rp.TestGroupID)
	oe.AddInt("group_instances", rp.TestGroupInstanceCount)

	if rp.TestRepo != "" {
		oe.AddString("repo", rp.TestRepo)
	}
	if rp.TestCommit != "" {
		oe.AddString("commit", rp.TestCommit)
	}
	if rp.TestBranch != "" {
		oe.AddString("branch", rp.TestBranch)
	}
	if rp.TestTag != "" {
		oe.AddString("tag", rp.TestTag)
	}
	return nil
}

// RecordMessage records an informational message.
func (l *logger) RecordMessage(msg string, a ...interface{}) {
	if len(a) > 0 {
		msg = fmt.Sprintf(msg, a...)
	}
	evt := Event{
		Type:    EventTypeMessage,
		Message: msg,
	}
	l.logger.Info("", zap.Object("event", evt))
}

func (l *logger) RecordStart() {
	evt := Event{
		Type:   EventTypeStart,
		Runenv: l.runenv,
	}

	l.logger.Info("", zap.Object("event", evt))
}

// RecordSuccess records that the calling instance succeeded.
func (l *logger) RecordSuccess() {
	evt := Event{
		Type:    EventTypeFinish,
		Outcome: EventOutcomeOK,
	}
	l.logger.Info("", zap.Object("event", evt))
}

// RecordFailure records that the calling instance failed with the supplied
// error.
func (l *logger) RecordFailure(err error) {
	evt := Event{
		Type:    EventTypeFinish,
		Outcome: EventOutcomeFailed,
		Error:   err.Error(),
	}
	l.logger.Info("", zap.Object("event", evt))
}

// RecordCrash records that the calling instance crashed/panicked with the
// supplied error.
func (l *logger) RecordCrash(err interface{}) {
	evt := Event{
		Type:       EventTypeFinish,
		Outcome:    EventOutcomeFailed,
		Error:      fmt.Sprintf("%s", err),
		Stacktrace: string(debug.Stack()),
	}
	l.logger.Error("", zap.Object("event", evt))
}
