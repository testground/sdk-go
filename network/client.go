package network

import (
	"context"
	"fmt"
	"os"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/testground/pkg/logging"
)

const (
	// magic values that we monitor on the Testground runner side to detect when Testground
	// testplan instances are initialised and at the stage of actually running a test
	// check cluster_k8s.go for more information
	InitialisationSuccessful = "network initialisation successful"
	InitialisationFailed     = "network initialisation failed"
)

type Client struct {
	runenv     *runtime.RunEnv
	syncClient sync.Client
}

// NewClient returns a new network client. Use this client to request network
// changes, such as setting latencies, jitter, packet loss, connectedness, etc.
func NewClient(syncClient sync.Client, runenv *runtime.RunEnv) *Client {
	return &Client{
		runenv:     runenv,
		syncClient: syncClient,
	}
}

// WaitNetworkInitialized waits for the sidecar to initialize the network, if
// the sidecar is enabled. If not, it returns immediately.
func (c *Client) WaitNetworkInitialized(ctx context.Context) error {
	fmt.Println("Starting wait")
	se := &runtime.Event{StageStartEvent: &runtime.StageStartEvent{
		Name:        "network-initialized",
		TestGroupID: c.runenv.TestGroupID,
	}}
	if err := c.syncClient.SignalEvent(ctx, se); err != nil {
		c.runenv.RecordMessage("Error signaling network-initialization start event: %s", err)
		return err
	}

	fmt.Printf("Runenv sidecar enabled: %t\n", c.runenv.TestSidecar)
	c.runenv.RecordMessage("Runenv sidecar enabled: %s", c.runenv.TestSidecar)

	if c.runenv.TestSidecar {
		c.runenv.RecordMessage("Waiting on network-initialized event")
		err := <-c.syncClient.MustBarrier(ctx, "network-initialized", c.runenv.TestInstanceCount).C
		if err != nil {
			c.runenv.RecordMessage(InitialisationFailed)
			return fmt.Errorf("failed to initialize network: %w", err)
		}
	}
	c.runenv.RecordMessage(InitialisationSuccessful)

	logging.S().Infof("Signaling sync client")
	ee := &runtime.Event{StageEndEvent: &runtime.StageEndEvent{
		Name:        "network-initialized",
		TestGroupID: c.runenv.TestGroupID,
	}}
	if err := c.syncClient.SignalEvent(ctx, ee); err != nil {
		logging.S().Errorf("Error signaling sync client: %s")
		c.runenv.RecordMessage("Error signaling network-initialization end event: %s", err)
		return err
	}
	c.runenv.RecordMessage("Finished waiting for network initialization")
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

	_, err = c.syncClient.PublishAndWait(ctx, topic, config, config.CallbackState, target)
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
