package runtime

type Notification struct {
	GroupID   string `json:"group_id"`
	EventType string `json:"event_type"` // start, end, failure, crash, entry, exit for stages
	Scope     string `json:"scope"`      // test case, test case stage
	StageName string `json:"stage_name"`
}
