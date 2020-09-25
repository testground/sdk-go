package network

import (
	"context"
	"fmt"
	"os"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

const (
	// magic values that we monitor on the Testground runner side to detect when Testground
	// testplan instances are initialised and at the stage of actually running a test
	// check cluster_k8s.go for more information
	InitialisationSuccessful = "network initialisation successful"
	InitialisationFailed     = "network initialisation failed"
)

type Client struct {
	runenv *runtime.RunEnv
	client sync.Client
}

// NewClient returns a new network client. Use this client to request network
// changes, such as setting latencies, jitter, packet loss, connectedness, etc.
func NewClient(client sync.Client, runenv *runtime.RunEnv) *Client {
	return &Client{
		runenv: runenv,
		client: client,
	}
}

// WaitNetworkInitialized waits for the sidecar to initialize the network, if
// the sidecar is enabled. If not, it returns immediately.
func (c *Client) WaitNetworkInitialized(ctx context.Context) error {
	synccl, ok := c.client.(*sync.DefaultClient)
	if ok {
		err := synccl.SignalEvent(ctx, &runtime.Notification{GroupID: c.runenv.TestGroupID, Scope: "stage", EventType: "entry", StageName: "network-initialized"})
		if err != nil {
			return err
		}
	}

	if c.runenv.TestSidecar {
		err := <-c.client.MustBarrier(ctx, "network-initialized", c.runenv.TestInstanceCount).C
		if err != nil {
			c.runenv.RecordMessage(InitialisationFailed)
			return fmt.Errorf("failed to initialize network: %w", err)
		}
	}
	c.runenv.RecordMessage(InitialisationSuccessful)

	if ok {
		err := synccl.SignalEvent(ctx, &runtime.Notification{GroupID: c.runenv.TestGroupID, Scope: "stage", EventType: "exit", StageName: "network-initialized"})
		if err != nil {
			return err
		}
	}

	return nil
}

// MustWaitNetworkInitialized calls WaitNetworkInitialized, and panics if it
// errors. It is suitable to use with runner.Invoke/InvokeMap, as long as
// this method is called from the main goroutine of the test plan.
func (c *Client) MustWaitNetworkInitialized(ctx context.Context) {
	err := c.WaitNetworkInitialized(ctx)
	if err != nil {
		panic(err)
	}
}

// ConfigureNetwork asks the sidecar to configure the network, and returns
// either when the sidecar signals back to us, or when the context expires.
func (c *Client) ConfigureNetwork(ctx context.Context, config *Config) (err error) {
	if !c.runenv.TestSidecar {
		msg := "ignoring network change request; running in a sidecar-less environment"
		c.runenv.SLogger().Named("netclient").Warn(msg)
		return nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to configure network; could not obtain hostname: %w", err)
	}

	if config.CallbackState == "" {
		return fmt.Errorf("failed to configure network; no callback state provided")
	}

	topic := sync.NewTopic("network:"+hostname, &Config{})

	target := config.CallbackTarget
	if target == 0 {
		// Fall back to instance count on zero value.
		target = c.runenv.TestInstanceCount
	}

	_, err = c.client.PublishAndWait(ctx, topic, config, config.CallbackState, target)
	if err != nil {
		err = fmt.Errorf("failed to configure network: %w", err)
	}
	return err
}

// MustConfigureNetwork calls ConfigureNetwork, and panics if it
// errors. It is suitable to use with runner.Invoke/InvokeMap, as long as
// this method is called from the main goroutine of the test plan.
func (c *Client) MustConfigureNetwork(ctx context.Context, config *Config) {
	err := c.ConfigureNetwork(ctx, config)
	if err != nil {
		panic(err)
	}
}
