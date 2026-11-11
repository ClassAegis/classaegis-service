package core

import (
	"fmt"
	"log"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// Agent wires config, executor, dedup set, dispatcher and MQTT client.
type Agent struct {
	cfg        *Config
	osName     string
	seen       *SeenSet
	client     paho.Client
	dispatcher *Dispatcher
	heartbeat  *time.Ticker
}

func NewAgent(cfg *Config, exec Executor, osName string) (*Agent, error) {
	seen, err := NewSeenSet(500)
	if err != nil {
		return nil, err
	}
	a := &Agent{cfg: cfg, osName: osName, seen: seen}
	a.dispatcher = NewDispatcher(exec, seen, func() string {
		return cfg.Device.DeviceSecret
	}, a.publishReceipt)
	return a, nil
}

func (a *Agent) topics() (down, up, hb, lwt string) {
	base := fmt.Sprintf("oneclassmesh/%s/%s", a.cfg.Device.SchoolCode, a.cfg.Device.SN)
	return base + "/cmd/down", base + "/cmd/up", base + "/hb", base + "/lwt"
}

// Run connects and blocks forever (console mode). Stop from another goroutine.
func (a *Agent) Run() error {
	down, _, hb, lwt := a.topics()
	addr := fmt.Sprintf("tcp://%s:%d", a.cfg.Broker.Host, a.cfg.Broker.Port)

	lwtPayload := `{"msgId":null,"status":"OFFLINE","ts":0}`

	opts := paho.NewClientOptions().
		AddBroker(addr).
		SetClientID(a.cfg.Device.SN).
		SetUsername(a.cfg.Device.SN).
		SetPassword(a.cfg.Device.MqttToken).
		SetCleanSession(false).
		SetKeepAlive(30*time.Second).
		SetAutoReconnect(true).
		SetOrderMatters(false).
		SetWriteTimeout(5*time.Second).
		SetWill(lwt, lwtPayload, 1, false)

	opts.SetOnConnectHandler(func(c paho.Client) {
		log.Printf("[agent] connected, subscribing %s", down)
		if tok := c.Subscribe(down, 1, func(_ paho.Client, m paho.Message) {
			a.dispatcher.OnCommand(string(m.Payload()))
		}); tok.Error() != nil {
			log.Printf("[agent] subscribe failed: %v", tok.Error())
		}
		a.startHeartbeat(hb)
	})

	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		log.Printf("[agent] connection lost: %v", err)
		a.stopHeartbeat()
	})

	client := paho.NewClient(opts)
	a.client = client
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		return fmt.Errorf("mqtt connect failed: %w", tok.Error())
	}
	log.Printf("[agent] running, broker=%s sn=%s", addr, a.cfg.Device.SN)

	select {} // block forever
}

func (a *Agent) publishReceipt(jsonStr string) {
	_, up, _, _ := a.topics()
	tok := a.client.Publish(up, 1, false, jsonStr)
	if tok.Wait() && tok.Error() != nil {
		log.Printf("[agent] receipt publish failed: %v", tok.Error())
	}
}

// PublishProgress publishes a non-terminal PROGRESS receipt for a running
// command (used by the all-in-one debug mode; the multi-process mode goes
// through core POST /event instead).
func (a *Agent) PublishProgress(msgId string, result map[string]any) {
	jsonStr, err := BuildReceipt(msgId, "PROGRESS", result, "", "")
	if err != nil {
		log.Printf("[agent] progress marshal failed: %v", err)
		return
	}
	a.publishReceipt(jsonStr)
}

func (a *Agent) startHeartbeat(topic string) {
	a.stopHeartbeat()
	seconds := a.cfg.Agent.HeartbeatSeconds
	if seconds <= 0 {
		seconds = 60
	}
	a.heartbeat = time.NewTicker(time.Duration(seconds) * time.Second)
	go func() {
		a.publishHeartbeat(topic)
		for range a.heartbeat.C {
			a.publishHeartbeat(topic)
		}
	}()
}

func (a *Agent) publishHeartbeat(topic string) {
	m := map[string]any{
		"msgId":           nil,
		"sn":              a.cfg.Device.SN,
		"protocolVersion": "0.1",
		"agentVersion":    "0.1.0",
		"status":          "ONLINE",
		"metrics":         map[string]any{"os": a.osName},
		"ts":              NowMillis(),
	}
	payload, err := MarshalCanonical(m)
	if err != nil {
		return
	}
	tok := a.client.Publish(topic, 0, false, payload)
	_ = tok
}

func (a *Agent) stopHeartbeat() {
	if a.heartbeat != nil {
		a.heartbeat.Stop()
	}
}

func (a *Agent) Stop() {
	a.stopHeartbeat()
	if a.client != nil && a.client.IsConnectionOpen() {
		a.client.Disconnect(1000)
	}
}
