// +build integration

package test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	"github/bherbruck/bromq/hooks/bridge"
	"github/bherbruck/bromq/internal/storage"
)

// TestBridgeIntegration tests end-to-end bridge functionality
// Run with: go test -tags=integration ./test/
func TestBridgeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database for test
	tempDB := "/tmp/mqtt-bridge-test.db"
	defer os.Remove(tempDB)

	dbConfig := storage.DefaultSQLiteConfig(tempDB)
	db, err := storage.Open(dbConfig)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Setup two MQTT servers
	// Server 1 (Remote): Acts as the remote broker
	remoteServer := setupTestServer(t, ":21883", ":0")
	defer remoteServer.Close()

	// Server 2 (Local): Bridges to remote server
	localServer := setupTestServer(t, ":21884", ":0")
	defer localServer.Close()

	// Create bridge configuration in database
	bridgeTopics := []storage.BridgeTopic{
		{
			LocalPattern:  "local/#",
			RemotePattern: "remote/#",
			Direction:     "out",
			QoS:           0,
		},
		{
			LocalPattern:  "inbound/#",
			RemotePattern: "from-remote/#",
			Direction:     "in",
			QoS:           0,
		},
	}

	testBridge, err := db.CreateBridge(
		"test-bridge",
		"localhost",
		21883, // Remote server port
		"",
		"",
		"test-bridge-client",
		true,
		30,
		10,
		nil,
		bridgeTopics,
	)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	// Start bridge manager
	bridgeManager := bridge.NewManager(db, localServer)
	if err := bridgeManager.Start(); err != nil {
		t.Fatalf("Failed to start bridge manager: %v", err)
	}
	defer bridgeManager.Stop()

	// Add bridge hook to local server
	bridgeHook := bridge.NewBridgeHook(bridgeManager)
	if err := localServer.AddHook(bridgeHook, nil); err != nil {
		t.Fatalf("Failed to add bridge hook: %v", err)
	}

	// Wait for bridge to connect
	time.Sleep(2 * time.Second)

	t.Run("Outbound", func(t *testing.T) {
		testOutboundBridge(t, testBridge)
	})

	t.Run("Inbound", func(t *testing.T) {
		testInboundBridge(t, testBridge)
	})

	t.Run("Bidirectional", func(t *testing.T) {
		testBidirectionalBridge(t, testBridge)
	})
}

func setupTestServer(t *testing.T, tcpAddr, wsAddr string) *mqttServer.Server {
	opts := &mqttServer.Options{
		InlineClient: true,
		Capabilities: mqttServer.NewDefaultServerCapabilities(),
	}
	// Maximize capabilities for testing
	opts.Capabilities.MaximumQos = 2
	opts.Capabilities.RetainAvailable = 1

	server := mqttServer.New(opts)

	// Add Allow All hook to permit anonymous connections
	server.AddHook(new(AllowHook), nil)

	// Add TCP listener
	tcp := listeners.NewTCP(listeners.Config{
		ID:      fmt.Sprintf("tcp-%s", tcpAddr),
		Address: tcpAddr,
	})
	if err := server.AddListener(tcp); err != nil {
		t.Fatalf("Failed to add TCP listener: %v", err)
	}

	// Start server in background
	go func() {
		if err := server.Serve(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond) // Wait for server to start

	return server
}

// AllowHook allows all connections (for testing)
type AllowHook struct {
	mqttServer.HookBase
}

func (h *AllowHook) ID() string {
	return "allow-all"
}

func (h *AllowHook) Provides(b byte) bool {
	return b == mqttServer.OnConnectAuthenticate || b == mqttServer.OnACLCheck
}

func (h *AllowHook) OnConnectAuthenticate(cl *mqttServer.Client, pk packets.Packet) bool {
	return true // Allow all connections
}

func (h *AllowHook) OnACLCheck(cl *mqttServer.Client, topic string, write bool) bool {
	return true // Allow all operations
}

func testOutboundBridge(t *testing.T, testBridge *storage.Bridge) {
	// Subscribe to remote broker
	remoteSub := setupMQTTClient(t, "tcp://localhost:21883", "remote-sub")
	defer remoteSub.Disconnect(250)

	received := make(chan string, 1)
	token := remoteSub.Subscribe("remote/#", 0, func(client mqtt.Client, msg mqtt.Message) {
		received <- string(msg.Payload())
	})
	token.Wait()
	if err := token.Error(); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish to local broker
	localPub := setupMQTTClient(t, "tcp://localhost:21884", "local-pub")
	defer localPub.Disconnect(250)

	testMsg := fmt.Sprintf("outbound-test-%d", time.Now().Unix())
	token = localPub.Publish("local/test", 0, false, testMsg)
	token.Wait()
	if err := token.Error(); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Wait for message to arrive at remote
	select {
	case msg := <-received:
		if msg != testMsg {
			t.Errorf("Expected %q, got %q", testMsg, msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for outbound message")
	}
}

func testInboundBridge(t *testing.T, testBridge *storage.Bridge) {
	// Subscribe to local broker
	localSub := setupMQTTClient(t, "tcp://localhost:21884", "local-sub")
	defer localSub.Disconnect(250)

	received := make(chan string, 1)
	token := localSub.Subscribe("inbound/#", 0, func(client mqtt.Client, msg mqtt.Message) {
		received <- string(msg.Payload())
	})
	token.Wait()
	if err := token.Error(); err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish to remote broker
	remotePub := setupMQTTClient(t, "tcp://localhost:21883", "remote-pub")
	defer remotePub.Disconnect(250)

	testMsg := fmt.Sprintf("inbound-test-%d", time.Now().Unix())
	token = remotePub.Publish("from-remote/test", 0, false, testMsg)
	token.Wait()
	if err := token.Error(); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Wait for message to arrive at local
	select {
	case msg := <-received:
		if msg != testMsg {
			t.Errorf("Expected %q, got %q", testMsg, msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for inbound message")
	}
}

func testBidirectionalBridge(t *testing.T, testBridge *storage.Bridge) {
	// Test rapid bidirectional messaging
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messageCount := 10
	receivedOut := make(chan string, messageCount)
	receivedIn := make(chan string, messageCount)

	// Remote subscriber for outbound messages
	remoteSub := setupMQTTClient(t, "tcp://localhost:21883", "remote-bidirectional")
	defer remoteSub.Disconnect(250)
	token := remoteSub.Subscribe("remote/#", 0, func(client mqtt.Client, msg mqtt.Message) {
		receivedOut <- string(msg.Payload())
	})
	token.Wait()

	// Local subscriber for inbound messages
	localSub := setupMQTTClient(t, "tcp://localhost:21884", "local-bidirectional")
	defer localSub.Disconnect(250)
	token = localSub.Subscribe("inbound/#", 0, func(client mqtt.Client, msg mqtt.Message) {
		receivedIn <- string(msg.Payload())
	})
	token.Wait()

	// Publishers
	localPub := setupMQTTClient(t, "tcp://localhost:21884", "local-publisher")
	defer localPub.Disconnect(250)
	remotePub := setupMQTTClient(t, "tcp://localhost:21883", "remote-publisher")
	defer remotePub.Disconnect(250)

	// Send messages in both directions
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < messageCount; i++ {
			localPub.Publish("local/data", 0, false, fmt.Sprintf("msg-out-%d", i))
			time.Sleep(50 * time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < messageCount; i++ {
			remotePub.Publish("from-remote/data", 0, false, fmt.Sprintf("msg-in-%d", i))
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// Wait for all sends
	wg.Wait()

	// Verify all messages received
	outCount := 0
	inCount := 0

	timeout := time.After(5 * time.Second)
	for outCount < messageCount || inCount < messageCount {
		select {
		case <-receivedOut:
			outCount++
		case <-receivedIn:
			inCount++
		case <-timeout:
			t.Fatalf("Timeout: received %d/%d outbound, %d/%d inbound",
				outCount, messageCount, inCount, messageCount)
		case <-ctx.Done():
			t.Fatal("Context cancelled")
		}
	}

	t.Logf("Successfully received all messages: %d outbound, %d inbound",
		outCount, inCount)
}

func setupMQTTClient(t *testing.T, broker, clientID string) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetConnectTimeout(2 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		t.Fatalf("Failed to connect to %s: %v", broker, err)
	}

	return client
}
