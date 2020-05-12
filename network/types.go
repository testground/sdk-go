package network

import (
	"fmt"
	"net"
	"time"

	"github.com/testground/sdk-go/sync"
)

// ErrNoTrafficShaping is returned from functions in this package when traffic
// shaping is not available, such as when using the local:exec runner.
var ErrNoTrafficShaping = fmt.Errorf("no traffic shaping available with this runner")

type FilterAction int

const (
	Accept FilterAction = iota
	Reject
	Drop
)

const (
	// The `data` network that Testground currently configures. In the future we might want to have
	// multiple `data` networks, and shape the network traffic on them differently.
	DefaultDataNetwork = "default"
)

// LinkShape defines how traffic should be shaped.
type LinkShape struct {
	// Latency is the egress latency
	Latency time.Duration

	// Jitter is the egress jitter
	Jitter time.Duration

	// Bandwidth is egress bytes per second
	Bandwidth uint64

	// Drop all inbound traffic.
	// TODO: Not implemented
	Filter FilterAction

	// Loss is the egress packet loss (%)
	Loss float32

	// Corrupt is the egress packet corruption probability (%)
	Corrupt float32

	// Corrupt is the egress packet corruption correlation (%)
	CorruptCorr float32

	// Reorder is the probability that an egress packet will be reordered (%)
	//
	// Reordered packets will skip the latency delay and be sent
	// immediately. You must specify a non-zero Latency for this option to
	// make sense.
	Reorder float32

	// ReorderCorr is the egress packet reordering correlation (%)
	ReorderCorr float32

	// Duplicate is the percentage of packets that are duplicated (%)
	Duplicate float32

	// DuplicateCorr is the correlation between egress packet duplication (%)
	DuplicateCorr float32
}

// LinkRule applies a LinkShape to a subnet.
type LinkRule struct {
	LinkShape
	Subnet net.IPNet
}

// NetworkConfig specifies how a node's network should be configured.
type Config struct {
	// Network is the name of the network to configure
	Network string

	// IPv4 and IPv6 set the IP addresses of this network device. If
	// unspecified, the sidecar will leave them alone.
	//
	// Your test-case will be assigned a B block in the range
	// 16.0.0.1-32.0.0.0. X.Y.0.1 will always be reserved for the gateway
	// and shouldn't be used by the test.
	//
	// TODO: IPv6 is currently not supported.
	IPv4, IPv6 *net.IPNet

	// Enable enables this network device.
	Enable bool

	// Default is the default link shaping rule.
	Default LinkShape

	// Rules defines how traffic should be shaped to different subnets.
	//
	// TODO: This is not implemented.
	Rules []LinkRule

	// CallbackState will be signalled when the link changes are applied.
	//
	// Nodes can use the same state to wait for _all_ or a subset of nodes to
	// enter the desired network state. See CallbackTarget.
	CallbackState sync.State `json:"State"`

	// CallbackTarget is the amount of instances that will have needed to signal
	// on the Callback state to consider the configuration operation a success.
	//
	// A zero value falls back to runenv.TestInstanceCount (i.e. all instances
	// participating in the test run).
	CallbackTarget int `json:"-"`
}
