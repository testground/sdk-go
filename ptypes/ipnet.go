package ptypes

import (
	"encoding/json"
	"net"
)

type IPNet struct {
	net.IPNet
}

func (i IPNet) MarshalJSON() ([]byte, error) {
	if len(i.IPNet.IP) == 0 {
		return json.Marshal("")
	}
	return json.Marshal(i.String())
}

func (i *IPNet) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	if s == "" {
		return nil
	}

	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return err
	}

	i.IPNet = *ipnet
	return nil
}
