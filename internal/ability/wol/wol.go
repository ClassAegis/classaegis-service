// Package wol implements the LAN wake/discovery ability (局域网唤醒/发现):
// WOL magic packets, subnet discovery and ARP snapshots. The process is
// loopback-only — network operations are delegated to core via the
// /netexec proxy; all validation lives here.
package wol

import (
	"errors"
	"log"

	"github.com/OneClassMesh/oneclassmesh-agent/internal/core/netops"
)

// NetExecFunc delegates one network operation to core. Injectable for
// tests; production uses ability.PostNetExec over loopback.
type NetExecFunc func(op string, args map[string]any) (map[string]any, error)

// Options configures the wol ability.
type Options struct {
	NetExec NetExecFunc // required; loopback proxy to core
	DefPort int         // default magic packet port; 0 = 9
}

// Engine is the LAN wake/discovery ability.
type Engine struct {
	opts Options
}

// New creates the engine.
func New(opts Options) *Engine {
	if opts.DefPort == 0 {
		opts.DefPort = 9
	}
	return &Engine{opts: opts}
}

// Name implements ability.Ability.
func (e *Engine) Name() string { return "wol" }

// Types implements ability.Ability; documented in the protocol spec §6.7.
func (e *Engine) Types() []string {
	return []string{"WOL_WAKE", "WOL_DISCOVER", "WOL_ARP"}
}

// Handle routes one wol command. No long tasks beyond the delegated probes.
func (e *Engine) Handle(msgId, msgType string, payload map[string]any, prog func(map[string]any)) (bool, string, string, map[string]any) {
	switch msgType {
	case "WOL_WAKE":
		return e.handleWake(payload)
	case "WOL_DISCOVER":
		return e.handleDiscover(payload)
	case "WOL_ARP":
		return e.handleARP(payload)
	default:
		return false, "E_UNSUPPORTED_TYPE", "unknown type: " + msgType, nil
	}
}

// netExec lazily defaults the NetExecFunc (nil-safe for tests).
func (e *Engine) netExec() NetExecFunc {
	if e.opts.NetExec == nil {
		return func(op string, args map[string]any) (map[string]any, error) {
			return nil, errors.New("netexec proxy unavailable")
		}
	}
	return e.opts.NetExec
}

// handleWake validates MAC input and delegates the magic packet send.
func (e *Engine) handleWake(payload map[string]any) (bool, string, string, map[string]any) {
	args := map[string]any{}
	if m, ok := payload["mac"].(string); ok && m != "" {
		args["mac"] = m
	}
	if raw, ok := payload["macs"].([]any); ok {
		args["macs"] = raw
	}
	if port, ok := payload["port"]; ok {
		args["port"] = port
	}
	if _, ok := args["mac"]; !ok {
		if _, ok := args["macs"]; !ok {
			return false, "E_PARAM", "mac or macs is required", nil
		}
	}
	// local pre-validation: fail fast before the proxy round-trip
	for _, key := range []string{"mac", "macs"} {
		switch v := args[key].(type) {
		case string:
			if _, err := netops.ParseMAC(v); err != nil {
				return false, "E_PARAM", err.Error(), nil
			}
		case []any:
			for _, item := range v {
				m, _ := item.(string)
				if _, err := netops.ParseMAC(m); err != nil {
					return false, "E_PARAM", err.Error(), nil
				}
			}
		}
	}
	if _, ok := args["port"]; !ok && e.opts.DefPort != 9 {
		args["port"] = e.opts.DefPort
	}
	res, err := e.netExec()("wol", args)
	if err != nil {
		log.Printf("[wol] wake failed: %v", err)
		return false, "E_FAILED", "wake failed: " + err.Error(), nil
	}
	return true, "", "", res
}

// handleDiscover delegates the subnet probe.
func (e *Engine) handleDiscover(payload map[string]any) (bool, string, string, map[string]any) {
	args := map[string]any{}
	if c, ok := payload["cidr"].(string); ok && c != "" {
		args["cidr"] = c
	}
	if t, ok := payload["timeout_ms"]; ok {
		args["timeout_ms"] = t
	}
	res, err := e.netExec()("discover", args)
	if err != nil {
		log.Printf("[wol] discover failed: %v", err)
		return false, "E_FAILED", "discover failed: " + err.Error(), nil
	}
	return true, "", "", res
}

// handleARP delegates the ARP snapshot.
func (e *Engine) handleARP(payload map[string]any) (bool, string, string, map[string]any) {
	res, err := e.netExec()("arp", map[string]any{})
	if err != nil {
		log.Printf("[wol] arp failed: %v", err)
		return false, "E_FAILED", "arp failed: " + err.Error(), nil
	}
	return true, "", "", res
}
