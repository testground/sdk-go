package runtime

type Notification struct {
	InstanceID   string `json:"instance_id"`
	GlobalSeqNum string `json:"global_seq_num"`
	EventType    string `json:"event_tyle"` // start, end, failure, crash, entry, exit for stages
	Scope        string `json:"scope"`      // test case, test case stage
	StageName    string `json:"stage_name"`
}
