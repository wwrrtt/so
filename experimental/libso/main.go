// Package main provides a C-compatible shared library (libsingbox.so) wrapping sing-box.
//
// Build:
//
//	go build -buildmode=c-shared -trimpath \
//	  -tags "with_cloudflared,with_quic,with_wireguard,with_utls,with_clash_api" \
//	  -ldflags "-s -w -buildid=" \
//	  -o libsingbox.so ./experimental/libso
//
// The resulting .so exports a handful of C functions (see header below).  Language
// bindings (Python ctypes, .NET P/Invoke, Node FFI, etc.) can call those directly —
// no separate binary process is needed.
//
// Minimal build tags
//
// To keep the .so small we omit non-essential features:
//
//	INCLUDED              OMITTED
//	- SOCKS / HTTP / mixed  - WireGuard endpoint
//	- Shadowsocks / VMess    - Tailscale
//	- Trojan / VLESS         - DHCP
//	- Hysteria / Hysteria2   - Naive outbound
//	- AnyTLS / ShadowTLS     - Tor / SSH
//	- TUIC / Snell           - ACME / CCM / OCM
//	- Direct / Block / DNS   - USB/IP
//	- cloudflared ✓          - v2ray API
//	- QUIC transports         - bridge outbound
//	- uTLS fingerprinting
//	- gVisor tun stack
//	- Clash API
//
// If you need additional protocols rebuild with the corresponding tags (see include/*.go).
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"context"
	"sync"
	"unsafe"

	box "github.com/sagernet/sing-box"
	C2 "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"
)

// ---------------------------------------------------------------------------
// Internal state
// ---------------------------------------------------------------------------

var (
	globalBox  *boxInstance
	globalLock sync.Mutex
)

type boxInstance struct {
	b      *box.Box
	cancel context.CancelFunc
}

// ---------------------------------------------------------------------------
// Exported C API
// ---------------------------------------------------------------------------

//export singbox_version
// singbox_version returns the sing-box core version string.
// The caller must free the result with singbox_free_string().
func singbox_version() *C.char {
	return C.CString(C2.Version)
}

//export singbox_start
// singbox_start creates and starts a sing-box instance from a JSON
// configuration string.
//
// Returns NULL on success; on failure returns a human-readable error
// message.  In either case the caller must free the returned pointer
// with singbox_free_string().
func singbox_start(configJSON *C.char) *C.char {
	globalLock.Lock()
	defer globalLock.Unlock()

	if globalBox != nil {
		return C.CString("sing-box is already running; stop it first")
	}

	raw := C.GoString(configJSON)

	ctx := context.Background()
	ctx = include.Context(ctx)
	ctx = service.ContextWith(ctx, deprecated.NewStderrManager(log.StdLogger()))

	opts, err := json.UnmarshalExtendedContext[option.Options](ctx, []byte(raw))
	if err != nil {
		return C.CString("config parse: " + err.Error())
	}

	// Sensible defaults when the caller omits top-level sections.
	if opts.Log == nil {
		opts.Log = &option.LogOptions{Level: "info"}
	}

	ctx, cancel := context.WithCancel(ctx)

	instance, err := box.New(box.Options{Context: ctx, Options: opts})
	if err != nil {
		cancel()
		return C.CString("create: " + err.Error())
	}

	if err = instance.Start(); err != nil {
		cancel()
		instance.Close()
		return C.CString("start: " + err.Error())
	}

	globalBox = &boxInstance{b: instance, cancel: cancel}
	return nil
}

//export singbox_stop
// singbox_stop gracefully stops the running instance.
//
// Returns NULL on success; on failure returns an error message.
// The caller must free the returned pointer with singbox_free_string().
func singbox_stop() *C.char {
	globalLock.Lock()
	defer globalLock.Unlock()

	if globalBox == nil {
		return C.CString("sing-box is not running")
	}

	inst := globalBox
	globalBox = nil

	inst.cancel()
	if err := inst.b.Close(); err != nil {
		return C.CString("stop: " + err.Error())
	}
	return nil
}

//export singbox_is_running
// singbox_is_running returns 1 when an instance is active, 0 otherwise.
func singbox_is_running() C.int {
	globalLock.Lock()
	defer globalLock.Unlock()
	if globalBox != nil {
		return 1
	}
	return 0
}

//export singbox_format_config
// singbox_format_config pretty-prints a sing-box JSON configuration.
//
// The caller must free the returned pointer with singbox_free_string().
func singbox_format_config(configJSON *C.char) *C.char {
	raw := C.GoString(configJSON)

	ctx := context.Background()
	ctx = include.Context(ctx)

	opts, err := json.UnmarshalExtendedContext[option.Options](ctx, []byte(raw))
	if err != nil {
		return C.CString("error: " + err.Error())
	}

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(opts); err != nil {
		return C.CString("error: " + err.Error())
	}
	return C.CString(out.String())
}

//export singbox_check_config
// singbox_check_config validates a JSON configuration by creating (and
// immediately tearing down) a temporary instance.
//
// Returns NULL when the config is valid; otherwise returns an error
// message.  The caller must free the returned pointer with
// singbox_free_string().
func singbox_check_config(configJSON *C.char) *C.char {
	raw := C.GoString(configJSON)

	ctx := context.Background()
	ctx = include.Context(ctx)

	opts, err := json.UnmarshalExtendedContext[option.Options](ctx, []byte(raw))
	if err != nil {
		return C.CString("config parse: " + err.Error())
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	instance, err := box.New(box.Options{Context: ctx, Options: opts})
	if err != nil {
		return C.CString("config error: " + err.Error())
	}
	instance.Close()
	return nil
}

//export singbox_free_string
// singbox_free_string releases memory previously returned by any
// singbox_* function that returns a *char.
func singbox_free_string(s *C.char) {
	C.free(unsafe.Pointer(s))
}

func main() {
	// Required by c-shared build mode; never called.
}
