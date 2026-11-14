// SPDX-License-Identifier: AGPL-3.0-only
// ocmctl is a debug tool that signs and publishes commands to a device,
// and can listen for receipts. It stands in for the not-yet-built server.
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/core"
)

func main() {
	broker := flag.String("broker", "127.0.0.1:1883", "broker host:port")
	school := flag.String("school", "demo", "school code")
	sn := flag.String("sn", "WIN-001", "target device SN")
	secret := flag.String("secret", "dev-secret-001", "device secret for HMAC")
	msgType := flag.String("type", "LOCK_SCREEN", "command type")
	payload := flag.String("payload", "{}", "payload JSON")
	listen := flag.Bool("listen", false, "subscribe cmd/up and print receipts")
	flag.Parse()

	payloadMap, err := core.ParseLoose(*payload)
	if err != nil {
		log.Fatalf("invalid payload JSON: %v", err)
	}
	payloadStr, err := core.MarshalCanonical(payloadMap)
	if err != nil {
		log.Fatalf("canonical payload failed: %v", err)
	}

	msgId := newMsgId()
	ts := core.NowMillis()
	canonical := msgId + "\n" + *msgType + "\n" + payloadStr + "\n" + fmt.Sprintf("%d", ts)
	sig := core.Sign(*secret, canonical)

	message := map[string]any{
		"msgId":   msgId,
		"type":    *msgType,
		"payload": payloadMap,
		"ts":      ts,
		"sig":     sig,
	}
	messageStr, err := core.MarshalCanonical(message)
	if err != nil {
		log.Fatalf("marshal message failed: %v", err)
	}

	if *listen {
		opts := paho.NewClientOptions().AddBroker("tcp://" + *broker).SetClientID("ocmctl-listen")
		client := paho.NewClient(opts)
		if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
			log.Fatalf("connect failed: %v", tok.Error())
		}
		subTopic := fmt.Sprintf("oneclassmesh/%s/%s/cmd/up", *school, *sn)
		if tok := client.Subscribe(subTopic, 1, func(_ paho.Client, m paho.Message) {
			fmt.Printf("[receipt] %s\n", string(m.Payload()))
		}); tok.Wait() && tok.Error() != nil {
			log.Fatalf("subscribe failed: %v", tok.Error())
		}
		fmt.Printf("listening on %s ...\n", subTopic)
		select {}
	}

	topic := fmt.Sprintf("oneclassmesh/%s/%s/cmd/down", *school, *sn)
	opts := paho.NewClientOptions().AddBroker("tcp://" + *broker).SetClientID("ocmctl-send")
	client := paho.NewClient(opts)
	if tok := client.Connect(); tok.Wait() && tok.Error() != nil {
		log.Fatalf("connect failed: %v", tok.Error())
	}
	tok := client.Publish(topic, 1, false, messageStr)
	if tok.Wait() && tok.Error() != nil {
		log.Fatalf("publish failed: %v", tok.Error())
	}
	fmt.Printf("published to %s: %s\n", topic, messageStr)
	time.Sleep(500 * time.Millisecond)
}

func newMsgId() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
