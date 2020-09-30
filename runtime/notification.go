package runtime

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
)

type Notification struct {
	GroupID   string      `json:"group_id"`
	EventType string      `json:"event_type"`
	Event     interface{} `json:"event"`
}

type StageEvent struct {
	Name   string `json:"name"`
	Action string `json:"action"` // entry or exit
}

type OutcomeEvent struct {
	Outcome string `json:"outcome"` // ok
}

func NewStageNotification(groupId, name, action string) *Notification {
	return &Notification{
		GroupID:   groupId,
		EventType: "stage",
		Event: &StageEvent{
			Name:   name,
			Action: action,
		},
	}
}

func NewOutcomeOKNotification(groupId string) *Notification {
	return &Notification{
		GroupID:   groupId,
		EventType: "outcome",
		Event: &OutcomeEvent{
			Outcome: "ok",
		},
	}
}

func (n *Notification) ParseEvent() error {
	if n.EventType == "stage" {
		r := &StageEvent{}
		err := mapstructure.Decode(n.Event, r)
		if err != nil {
			return err
		}

		n.Event = r

		return nil
	}

	if n.EventType == "outcome" {
		r := &OutcomeEvent{}
		err := mapstructure.Decode(n.Event, r)
		if err != nil {
			return err
		}

		n.Event = r

		return nil
	}

	return fmt.Errorf("unknown type: %s", n.EventType)
}
