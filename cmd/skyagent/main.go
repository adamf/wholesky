// skyagent is the seat as MCP tools: a stdio server an agent runtime
// (Claude Code, Claude Desktop, anything that speaks MCP) starts, pointed
// at a wholesky world, through which a model runs a carrier -- reads the
// lobby and the scorecard, takes a seat, takes departments off autopilot,
// answers the decisions the day puts to it, pulls the levers. It is a thin
// wrapper over the same HTTP API the operations centre page uses, so
// nothing an agent can do is hidden from a person and nothing a person can
// do is beyond an agent.
//
//	skyagent -world https://wholesky-demo.fly.dev
//
// The world, the carrier and the seat token can also come from
// SKYAGENT_WORLD, SKYAGENT_CARRIER and SKYAGENT_TOKEN; the take_seat tool
// remembers the token for the session.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// seat is the session's connection to one world.
type seat struct {
	world  string
	client *http.Client
	mu     sync.Mutex
	code   string
	token  string
}

func (s *seat) call(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(s.world, "/")+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.mu.Lock()
	if s.token != "" {
		req.Header.Set("X-Seat-Token", s.token)
	}
	s.mu.Unlock()
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		// A list, not an object: wrap it.
		var list []any
		if err2 := json.Unmarshal(raw, &list); err2 == nil {
			out = map[string]any{"items": list}
		} else {
			return nil, fmt.Errorf("%s %s: %s", method, path, strings.TrimSpace(string(raw)))
		}
	}
	if resp.StatusCode >= 400 {
		if e, ok := out["error"].(string); ok {
			return nil, fmt.Errorf("%s", e)
		}
		return nil, fmt.Errorf("%s %s: HTTP %d", method, path, resp.StatusCode)
	}
	return out, nil
}

func (s *seat) carrier(given string) (string, error) {
	if given != "" {
		return strings.ToUpper(given), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.code == "" {
		return "", fmt.Errorf("no carrier: take a seat first, or name one")
	}
	return s.code, nil
}

func text(v any) *mcp.CallToolResult {
	b, _ := json.MarshalIndent(v, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

type carrierArg struct {
	Carrier string `json:"carrier,omitempty" jsonschema:"two-letter carrier code; the seat's own when omitted"`
}

type takeArgs struct {
	Carrier string `json:"carrier" jsonschema:"two-letter carrier code to run"`
	Holder  string `json:"holder" jsonschema:"who is taking the seat, e.g. the agent's name"`
}

type deptArgs struct {
	Carrier    string `json:"carrier,omitempty"`
	Department string `json:"department" jsonschema:"ops, crew, slots, pricing or ground"`
	Manual     bool   `json:"manual" jsonschema:"true takes the department off autopilot so the day asks you; false gives it back"`
}

type decideArgs struct {
	Carrier string `json:"carrier,omitempty"`
	ID      string `json:"id" jsonschema:"the decision id from the inbox"`
	Option  string `json:"option" jsonschema:"one of the decision's option keys"`
}

type actArgs struct {
	Carrier    string  `json:"carrier,omitempty"`
	Kind       string  `json:"kind" jsonschema:"cancel, retime, substitute, class, fares, ready or reserves"`
	Flight     string  `json:"flight,omitempty" jsonschema:"flight designator and number, e.g. BA0117"`
	Board      string  `json:"board,omitempty" jsonschema:"boarding point IATA code of the departure"`
	Minutes    int     `json:"minutes,omitempty" jsonschema:"retime: the delay to announce"`
	Class      string  `json:"class,omitempty" jsonschema:"class: the booking class letter"`
	Status     string  `json:"status,omitempty" jsonschema:"class: C to close, empty to return it to the ladder"`
	Multiplier float64 `json:"multiplier,omitempty" jsonschema:"fares: factor over the filing; 0 restores it"`
	Reason     string  `json:"reason,omitempty"`
}

// newServer builds the MCP server over one world.
func newServer(s *seat) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "skyagent", Version: "0.1"}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "lobby", Description: "The world's carriers with their scorecards, ranked, and who holds each seat. The clock is the sim day's minutes and the warp its speed."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			out, err := s.call(ctx, "GET", "/carriers.json", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "take_seat", Description: "Take a carrier: from now on you run it. Every department stays on autopilot until you take it manual. The token is kept for this session."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a takeArgs) (*mcp.CallToolResult, any, error) {
			out, err := s.call(ctx, "POST", "/carrier/"+strings.ToUpper(a.Carrier)+"/take", map[string]string{"holder": a.Holder})
			if err != nil {
				return nil, nil, err
			}
			s.mu.Lock()
			s.code = strings.ToUpper(a.Carrier)
			s.token, _ = out["token"].(string)
			s.mu.Unlock()
			delete(out, "token")
			out["note"] = "seat token kept for this session; every change now goes through it"
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "release_seat", Description: "Hand the carrier back to the autopilot. Open decisions fall to their defaults."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a carrierArg) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "POST", "/carrier/"+code+"/release", nil)
			if err != nil {
				return nil, nil, err
			}
			s.mu.Lock()
			s.code, s.token = "", ""
			s.mu.Unlock()
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "carrier_state", Description: "Everything about a carrier now: the scorecard (revenue, costs, profit, on-time, cancellations, load factor), every departure today with its status and what the day has done to it, the open decisions, and the departments."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a carrierArg) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "GET", "/carrier/"+code+"/state", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "inbox", Description: "The decisions the day is waiting on you for, each with its options, default and deadline. Unanswered decisions fall to the default at the deadline."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a carrierArg) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "GET", "/carrier/"+code+"/inbox", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "set_department", Description: "Take a department off autopilot (manual) or give it back. Manual departments put their decisions in your inbox: ops (delays, substitutions), crew (timed-out crews), slots (the Network Manager's slots), pricing, ground (short-shipped bags)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a deptArgs) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "POST", "/carrier/"+code+"/departments", map[string]any{"department": a.Department, "manual": a.Manual})
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "decide", Description: "Answer an open decision with one of its option keys."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a decideArgs) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "POST", "/carrier/"+code+"/decide", map[string]string{"id": a.ID, "option": a.Option})
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "act", Description: "Pull a lever now: cancel a flight (flight, board, reason), retime it (flight, board, minutes), substitute a smaller aircraft (flight, board), force a booking class closed or back (flight, board, class, status C or empty), move every fare by a multiplier (multiplier), send REA to ask the Network Manager for a better slot (flight, board), call reserves for a crew-timed-out flight (flight, board). Each goes out on the wire as the real messages."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a actArgs) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			body := map[string]any{"kind": a.Kind, "flight": a.Flight, "board": a.Board, "minutes": a.Minutes, "class": a.Class, "status": a.Status, "multiplier": a.Multiplier, "reason": a.Reason}
			out, err := s.call(ctx, "POST", "/carrier/"+code+"/act", body)
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "tape", Description: "The carrier's recent events: decisions opened and closed, actions, incidents the day threw at it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a carrierArg) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "GET", "/carrier/"+code+"/tape", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "pack", Description: "The start pack for a carrier the world does not run itself (booted with -external): the jetway node configuration to run as that carrier, the switch address, the link token, and the schedule as an SSIM file. Bring your own jetway."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a carrierArg) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "GET", "/carrier/"+code+"/pack", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "weather", Description: "The day's weather systems and the Network Manager's regulations: which airports are slowed, when, and by how much."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			out, err := s.call(ctx, "GET", "/dayplan.json", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	return srv
}

func main() {
	world := flag.String("world", envOr("SKYAGENT_WORLD", "http://localhost:8080"), "the wholesky world's URL")
	flag.Parse()
	s := &seat{world: *world, client: &http.Client{Timeout: 40 * time.Second}, code: strings.ToUpper(os.Getenv("SKYAGENT_CARRIER")), token: os.Getenv("SKYAGENT_TOKEN")}
	if err := newServer(s).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
