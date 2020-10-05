package runtime

import (
	"context"
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
	EventTypeOutcome = EventType("outcome")

	EventOutcomeOK      = EventOutcome("ok")
	EventOutcomeFailed  = EventOutcome("failed")
	EventOutcomeCrashed = EventOutcome("crashed")
)

type Event struct {
	*StartEvent      `json:"start_event"`
	*MessageEvent    `json:"message_event"`
	*SuccessEvent    `json:"success_event"`
	*FailureEvent    `json:"failure_event"`
	*CrashEvent      `json:"crash_event"`
	*StageStartEvent `json:"stage_start_event"`
	*StageEndEvent   `json:"stage_end_event"`
}

func (e *Event) Type() string {
	switch {
	case e.StartEvent != nil:
		return e.StartEvent.Type()
	case e.MessageEvent != nil:
		return e.MessageEvent.Type()
	case e.SuccessEvent != nil:
		return e.SuccessEvent.Type()
	case e.FailureEvent != nil:
		return e.FailureEvent.Type()
	case e.CrashEvent != nil:
		return e.CrashEvent.Type()
	case e.StageStartEvent != nil:
		return e.StageStartEvent.Type()
	case e.StageEndEvent != nil:
		return e.StageEndEvent.Type()
	default:
		panic("no such event")
	}
}

type StartEvent struct {
	Runenv *RunParams `json:"runenv"`
}

func (StartEvent) Type() string {
	return "StartEvent"
}

func (s StartEvent) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("type", s.Type())
	return oe.AddObject("runenv", s.Runenv)
}

type MessageEvent struct {
	Message string `json:"message"`
}

func (MessageEvent) Type() string {
	return "MessageEvent"
}

func (m MessageEvent) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("type", m.Type())
	oe.AddString("message", m.Message)
	return nil
}

type SuccessEvent struct {
	TestGroupID string `json:"group"`
}

func (SuccessEvent) Type() string {
	return "SuccessEvent"
}

func (s SuccessEvent) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("type", s.Type())
	oe.AddString("group", s.TestGroupID)
	return nil
}

type FailureEvent struct {
	Error string `json:"error"`
}

func (FailureEvent) Type() string {
	return "FailureEvent"
}

func (f FailureEvent) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("type", f.Type())
	oe.AddString("error", f.Error)
	return nil
}

type CrashEvent struct {
	Error      string `json:"error"`
	Stacktrace string `json:"stacktrace"`
}

func (CrashEvent) Type() string {
	return "CrashEvent"
}

func (c CrashEvent) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("type", c.Type())
	oe.AddString("error", c.Error)
	oe.AddString("stacktrace", c.Stacktrace)
	return nil
}

type StageStartEvent struct {
	Name        string `json:"name"`
	TestGroupID string `json:"group"`
}

func (StageStartEvent) Type() string {
	return "StageStartEvent"
}

func (s StageStartEvent) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("type", s.Type())
	oe.AddString("name", s.Name)
	oe.AddString("group", s.TestGroupID)
	return nil
}

type StageEndEvent struct {
	Name        string `json:"name"`
	TestGroupID string `json:"group"`
}

func (StageEndEvent) Type() string {
	return "StageEndEvent"
}

func (s StageEndEvent) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("type", s.Type())
	oe.AddString("name", s.Name)
	oe.AddString("group", s.TestGroupID)
	return nil
}

func (e Event) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	switch {
	case e.StartEvent != nil:
		return e.StartEvent.MarshalLogObject(oe)
	case e.MessageEvent != nil:
		return e.MessageEvent.MarshalLogObject(oe)
	case e.SuccessEvent != nil:
		return e.SuccessEvent.MarshalLogObject(oe)
	case e.FailureEvent != nil:
		return e.FailureEvent.MarshalLogObject(oe)
	case e.CrashEvent != nil:
		return e.CrashEvent.MarshalLogObject(oe)
	case e.StageStartEvent != nil:
		return e.StageStartEvent.MarshalLogObject(oe)
	case e.StageEndEvent != nil:
		return e.StageEndEvent.MarshalLogObject(oe)
	default:
		panic("no such event")
	}

	//oe.AddString("type", string(e.Type))

	//if e.Outcome != "" {
	//oe.AddString("outcome", string(e.Outcome))
	//}
	//if e.Error != "" {
	//oe.AddString("error", e.Error)
	//}
	//if e.Stacktrace != "" {
	//oe.AddString("stacktrace", e.Stacktrace)
	//}
	//if e.Message != "" {
	//oe.AddString("message", e.Message)
	//}
	//if e.Runenv != nil {
	//if err := oe.AddObject("runenv", e.Runenv); err != nil {
	//return err
	//}
	//}
}

func (rp *RunParams) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddString("plan", rp.TestPlan)
	oe.AddString("case", rp.TestCase)
	oe.AddString("run", rp.TestRun)
	if err := oe.AddReflected("params", rp.TestInstanceParams); err != nil {
		return err
	}
	oe.AddInt("instances", rp.TestInstanceCount)
	oe.AddString("outputs_path", rp.TestOutputsPath)
	oe.AddString("network", func() string {
		if rp.TestSubnet == nil {
			return ""
		}
		return rp.TestSubnet.String()
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
func (re *RunEnv) RecordMessage(msg string, a ...interface{}) {
	if len(a) > 0 {
		msg = fmt.Sprintf(msg, a...)
	}
	e := &Event{MessageEvent: &MessageEvent{
		Message: msg,
	}}
	re.logger.Info("", zap.Object("event", e))
}

func (re *RunEnv) RecordStart() {
	e := &Event{StartEvent: &StartEvent{
		Runenv: &re.RunParams,
	}}

	re.logger.Info("", zap.Object("event", e))
	re.metrics.recordEvent(e)
}

// RecordSuccess records that the calling instance succeeded.
func (re *RunEnv) RecordSuccess() {
	e := &Event{SuccessEvent: &SuccessEvent{TestGroupID: re.RunParams.TestGroupID}}
	re.logger.Info("", zap.Object("event", e))
	re.metrics.recordEvent(e)

	if re.signalEventer != nil {
		_ = re.signalEventer.SignalEvent(context.Background(), e)
	}
}

// RecordFailure records that the calling instance failed with the supplied
// error.
func (re *RunEnv) RecordFailure(err error) {
	e := &Event{FailureEvent: &FailureEvent{
		Error: err.Error(),
	}}
	re.logger.Info("", zap.Object("event", e))
	re.metrics.recordEvent(e)

	if re.signalEventer != nil {
		_ = re.signalEventer.SignalEvent(context.Background(), e)
	}
}

// RecordCrash records that the calling instance crashed/panicked with the
// supplied error.
func (re *RunEnv) RecordCrash(err interface{}) {
	e := &Event{CrashEvent: &CrashEvent{
		Error:      fmt.Sprintf("%s", err),
		Stacktrace: string(debug.Stack()),
	}}
	re.logger.Error("", zap.Object("event", e))
	re.metrics.recordEvent(e)

	if re.signalEventer != nil {
		_ = re.signalEventer.SignalEvent(context.Background(), e)
	}
}
