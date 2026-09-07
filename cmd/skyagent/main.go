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
	"strconv"
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
	// record, when set, gets one JSON line per call: what the agent asked
	// the world and what it answered, for the record.
	record *os.File
}

// logCall appends one line to the local record.
func (s *seat) logCall(method, path string, body any, status int, out map[string]any) {
	if s.record == nil {
		return
	}
	line := map[string]any{"t": time.Now().UTC().Format(time.RFC3339Nano), "method": method, "path": path, "status": status}
	if body != nil {
		line["body"] = body
	}
	if method != http.MethodGet {
		line["result"] = out
	}
	b, err := json.Marshal(line)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.record.Write(append(b, '\n')) //nolint:errcheck
	s.mu.Unlock()
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
	defer func() { s.logCall(method, path, body, resp.StatusCode, out) }()
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

type noteArgs struct {
	Carrier string `json:"carrier,omitempty" jsonschema:"the carrier, when not the seat held"`
	Text    string `json:"text" jsonschema:"what you are thinking, a sentence or two"`
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

type nodeArgs struct {
	Carrier string `json:"carrier,omitempty"`
	URL     string `json:"url" jsonschema:"where the node's console answers, e.g. http://localhost:8080; empty forgets it"`
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
	mcp.AddTool(srv, &mcp.Tool{Name: "lobby", Description: "The carriers in the world with their scorecards, ranked, and the holder of each seat. pos is the sim clock in minutes of the day; warp is its speed."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
			out, err := s.call(ctx, "GET", "/carriers.json", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(compactLobby(out, s.held())), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "take_seat", Description: "Take a carrier. You run it until you release it. Every department stays on autopilot until you set it to manual. The seat token is kept for this session."},
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
			if seat, ok := out["seat"].(map[string]any); ok {
				if id, _ := seat["recording"].(string); id != "" {
					out["replay"] = strings.TrimRight(s.world, "/") + "/replay/" + id
					out["hint"] = "your run is being recorded; narrate it with the note tool as you go, and hand people the replay URL when you are done"
				}
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "release_seat", Description: "Return the carrier to the autopilot. Open decisions take their defaults."},
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
	mcp.AddTool(srv, &mcp.Tool{Name: "carrier_state", Description: "The carrier now: the scorecard (revenue, costs, profit, on-time, cancellations, load factor), the departures the day has affected and the next ones due, the open decisions, and the departments."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a carrierArg) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "GET", "/carrier/"+code+"/state", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(compactState(out)), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "inbox", Description: "The open decisions, each with its options, default and deadline. A decision that is not answered by its deadline takes the default."},
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
	mcp.AddTool(srv, &mcp.Tool{Name: "set_department", Description: "Set a department to manual or back to autopilot. A manual department sends its decisions to your inbox: ops (delays, substitutions), crew (timed-out crews), slots (Network Manager slots), pricing, ground (bags left behind)."},
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
	mcp.AddTool(srv, &mcp.Tool{Name: "act", Description: "Apply a lever now: cancel a flight (flight, board, reason), retime it (flight, board, minutes), substitute a smaller aircraft (flight, board), close a booking class or reopen it (flight, board, class, status C or empty), multiply the fares (multiplier), send REA to request a better slot (flight, board), call reserves for a flight whose crew has timed out (flight, board). Each lever sends the corresponding messages."},
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
	mcp.AddTool(srv, &mcp.Tool{Name: "note", Description: "Record what you are thinking: one or two sentences before or after a decision or an action, with what you saw, what you compared and why you chose. The note goes on the carrier's tape and into the replay of your run (take_seat returns the replay URL). Write a note for every decision and action."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a noteArgs) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "POST", "/carrier/"+code+"/note", map[string]string{"text": a.Text})
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "tape", Description: "The carrier's recent events: decisions opened and answered, actions, and incidents."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a carrierArg) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "GET", "/carrier/"+code+"/tape", nil)
			if err != nil {
				return nil, nil, err
			}
			if items, ok := out["items"].([]any); ok && len(items) > 40 {
				out["items"] = items[len(items)-40:]
				out["note"] = fmt.Sprintf("the last 40 of %d lines", len(items))
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "pack", Description: "The start pack for a carrier that the world does not run itself (started with -external): the jetway node configuration for that carrier, the switch address, the link token, and the schedule as an SSIM file."},
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
	mcp.AddTool(srv, &mcp.Tool{Name: "claim", Description: "Transfer the carrier you hold to your own jetway node. The world disconnects its tenant and returns the start pack (jetway YAML with the switch address and link token, and the SSIM schedule). Run jetwayd with the YAML and your node is the carrier. unclaim reverses this."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a carrierArg) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "POST", "/carrier/"+code+"/claim", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "unclaim", Description: "Return a claimed carrier to the world. Its tenant reconnects to the switch and the simulator runs it again."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a carrierArg) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "POST", "/carrier/"+code+"/unclaim", nil)
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "register_node", Description: "Give the world the HTTP URL of your jetway node (after claim). The world then drives your carrier's ground schedule: the name list 3 hours before each departure, and check-in, boarding and the load 45 minutes before. An empty url removes it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, a nodeArgs) (*mcp.CallToolResult, any, error) {
			code, err := s.carrier(a.Carrier)
			if err != nil {
				return nil, nil, err
			}
			out, err := s.call(ctx, "POST", "/carrier/"+code+"/node", map[string]string{"url": a.URL})
			if err != nil {
				return nil, nil, err
			}
			return text(out), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "weather", Description: "The weather systems and Network Manager regulations for the day: the airports with reduced rates, the times, and the fraction of the normal rate."},
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
	record := flag.String("record", os.Getenv("SKYAGENT_RECORD"), "append every call and its answer to this JSONL file, for the record")
	flag.Parse()
	s := &seat{world: *world, client: &http.Client{Timeout: 40 * time.Second}, code: strings.ToUpper(os.Getenv("SKYAGENT_CARRIER")), token: os.Getenv("SKYAGENT_TOKEN")}
	if *record != "" {
		f, err := os.OpenFile(*record, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			log.Fatal(err)
		}
		s.record = f
	}
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

// held is the carrier this session holds, if any.
func (s *seat) held() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.code
}

// The world answers with everything; a model reads a page. The compact
// views below keep what a seat decides on and count the rest: a lobby of
// five hundred carriers becomes the top of the table plus the seats held
// and one's own; a state with five hundred departures becomes the ones
// the day has touched, the next few to leave, and the totals.

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}

// compactLobby keeps the top of the table, every held seat and the
// caller's own carrier.
func compactLobby(out map[string]any, own string) map[string]any {
	rows, ok := out["carriers"].([]any)
	if !ok || len(rows) <= 40 {
		return out
	}
	var keep []any
	for i, r := range rows {
		m, _ := r.(map[string]any)
		if m == nil {
			continue
		}
		code, _ := m["code"].(string)
		if i < 30 || m["seat"] != nil || code == own {
			m["rank"] = i + 1
			keep = append(keep, m)
		}
	}
	out["carriers"] = keep
	out["note"] = fmt.Sprintf("%d of %d carriers: the top thirty, every seat held, and yours", len(keep), len(rows))
	return out
}

// compactState keeps the scorecard, the inbox and the departments whole,
// and of the flights the ones the day has touched plus the next dozen.
func compactState(out map[string]any) map[string]any {
	flights, ok := out["flights"].([]any)
	if !ok {
		return out
	}
	pos := num(out["pos"])
	byStatus := map[string]int{}
	var touched, upcoming []any
	for _, f := range flights {
		m, _ := f.(map[string]any)
		if m == nil {
			continue
		}
		st, _ := m["status"].(string)
		byStatus[st]++
		annotated := num(m["delay_min"]) >= 15
		for _, k := range []string{"slot", "crew", "retimed", "substituted", "rushed", "cancelled"} {
			if v, _ := m[k].(string); v != "" {
				annotated = true
			}
		}
		std, _ := m["std"].(string)
		var stdMin float64 = -1
		if len(std) == 4 {
			h, herr := strconv.Atoi(std[:2])
			mi, merr := strconv.Atoi(std[2:])
			if herr == nil && merr == nil {
				stdMin = float64(h*60 + mi)
			}
		}
		if annotated && st != "arrived" {
			touched = append(touched, m)
		} else if stdMin >= pos && stdMin < pos+180 && len(upcoming) < 12 {
			upcoming = append(upcoming, m)
		}
	}
	if len(touched) > 40 {
		touched = touched[len(touched)-40:]
	}
	out["flights"] = append(touched, upcoming...)
	out["flights_summary"] = map[string]any{"total": len(flights), "by_status": byStatus,
		"shown": fmt.Sprintf("%d the day has touched (delay 15+, slot, crew, retime, substitution, rush, cancellation) and %d leaving in the next three hours", len(touched), len(upcoming))}
	return out
}
