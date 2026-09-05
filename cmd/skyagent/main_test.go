package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The MCP server over a stand-in world: a client lists the tools, takes a
// seat (the token is kept and sent on every later call), reads the inbox
// and pulls a lever.
func TestToolsRunACarrierOverHTTP(t *testing.T) {
	var sawToken string
	world := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/carriers.json":
			json.NewEncoder(w).Encode(map[string]any{"carriers": []map[string]any{{"code": "BA", "score": map[string]any{"score": 12.5}}}, "pos": 600, "warp": 6}) //nolint:errcheck
		case strings.HasSuffix(r.URL.Path, "/take"):
			json.NewEncoder(w).Encode(map[string]any{"seat": map[string]any{"carrier": "BA"}, "token": "secret"}) //nolint:errcheck
		case strings.HasSuffix(r.URL.Path, "/inbox"):
			sawToken = r.Header.Get("X-Seat-Token")
			json.NewEncoder(w).Encode([]map[string]any{{"id": "d1", "title": "BA117 late"}}) //nolint:errcheck
		case strings.HasSuffix(r.URL.Path, "/act"):
			sawToken = r.Header.Get("X-Seat-Token")
			var a map[string]any
			json.NewDecoder(r.Body).Decode(&a) //nolint:errcheck
			if a["kind"] != "fares" {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "wrong kind"}) //nolint:errcheck
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"ok": "done", "result": "fares now ×1.10"}) //nolint:errcheck
		default:
			w.WriteHeader(404)
			json.NewEncoder(w).Encode(map[string]string{"error": "no route " + r.URL.Path}) //nolint:errcheck
		}
	}))
	defer world.Close()

	s := &seat{world: world.URL, client: &http.Client{Timeout: 5 * time.Second}}
	srv := newServer(s)
	ct, st := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go srv.Run(ctx, st) //nolint:errcheck
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	tools, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	for _, want := range []string{"lobby", "take_seat", "carrier_state", "inbox", "set_department", "decide", "act", "tape", "weather", "release_seat", "pack"} {
		if !names[want] {
			t.Errorf("no tool %s in %v", want, names)
		}
	}
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "lobby", Arguments: map[string]any{}})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "12.5") {
		t.Fatalf("lobby: %+v %v", res, err)
	}
	if _, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "take_seat", Arguments: map[string]any{"carrier": "ba", "holder": "claude"}}); err != nil {
		t.Fatal(err)
	}
	res, err = sess.CallTool(ctx, &mcp.CallToolParams{Name: "inbox", Arguments: map[string]any{}})
	if err != nil || res.IsError || sawToken != "secret" || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "BA117 late") {
		t.Fatalf("inbox: %+v %v token %q", res, err, sawToken)
	}
	res, err = sess.CallTool(ctx, &mcp.CallToolParams{Name: "act", Arguments: map[string]any{"kind": "fares", "multiplier": 1.1}})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "1.10") {
		t.Fatalf("act: %+v %v", res, err)
	}
	// A world error comes back as a tool error, not a crash.
	res, err = sess.CallTool(ctx, &mcp.CallToolParams{Name: "act", Arguments: map[string]any{"kind": "cancel", "flight": "BA0117"}})
	if err == nil && !res.IsError {
		t.Errorf("a refused action should be a tool error: %+v", res)
	}
}
