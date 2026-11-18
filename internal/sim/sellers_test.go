package sim

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/pnr"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/distribution"
	"github.com/adamf/wholesky/internal/market"
)

// Who sold the ticket. A traveller who buys from the carrier books on
// the carrier's own system: the record is the carrier's alone, the seat
// is held at once, and no distribution system hears of it. A traveller
// who buys from an agency is booked by a named agency, through its
// distribution system or over its direct connection to the carrier, and
// the ledger keeps what the agency sold and earned.
func TestWhoSoldTheTicket(t *testing.T) {
	s, carriers, ctx := bootQuiet(t)
	for i := 0; i < 400; i++ {
		s.placeDemand(ctx, s.GDSes[0], rand.New(rand.NewSource(int64(5000+i))), carriers, i)
		if s.DemDirect.Load() >= 5 && s.DemConnected.Load() >= 2 && s.DemBooked.Load() >= 40 {
			break
		}
	}
	t.Logf("booked %d: %d on carriers' own systems, %d over agencies' direct connections, %d refused, %d NDC orders",
		s.DemBooked.Load(), s.DemDirect.Load(), s.DemConnected.Load(), s.DemRefused.Load(), s.DemNDC.Load())
	if s.DemDirect.Load() == 0 || s.DemConnected.Load() == 0 {
		t.Fatal("no sale was made on a carrier's own system, or none over a direct connection")
	}
	// The carriers' books: their own sales, held, priced, and theirs alone.
	own, connected := 0, 0
	for code, tn := range s.Tenants {
		recs, err := tn.Store.ListPNRs(ctx, 10000)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range recs {
			if r.Origin.Party != code || r.Status == pnr.StatusCancelled {
				continue
			}
			if len(r.Locators) != 0 || r.Pricing == nil {
				t.Errorf("%s's own sale %s: locators %v, priced %v", code, r.RecordLocator, r.Locators, r.Pricing != nil)
			}
			for _, sg := range r.Segments {
				if sg.Type == pnr.SegmentAir && sg.Status != "HK" && sg.Status != "HL" && sg.Status != "XX" {
					t.Errorf("%s's own sale %s holds %s%s at %s", code, r.RecordLocator, sg.Carrier, sg.FlightNum, sg.Status)
				}
				if sg.Type == pnr.SegmentAir && sg.Carrier != code {
					t.Errorf("%s sold %s%s on its own system", code, sg.Carrier, sg.FlightNum)
				}
			}
			switch r.Origin.Channel {
			case market.Direct, "ndc":
				if _, agency := s.commerce.agency[r.Origin.Agent]; agency {
					connected++
				} else {
					own++
				}
			default:
				t.Errorf("%s's own sale %s is in the channel %q", code, r.RecordLocator, r.Origin.Channel)
			}
		}
	}
	if own == 0 || connected == 0 {
		t.Errorf("the carriers' books hold %d sales of their own and %d from connected agencies", own, connected)
	}
	// The distribution system's book: every sale by a named agency, or the
	// carrier's site where the carrier's own system could not take it.
	recs, _ := s.GDSStore.ListPNRs(ctx, 10000)
	byAgency := map[string]int{}
	for _, r := range recs {
		a, ok := s.commerce.agency[r.Origin.Agent]
		switch {
		case ok:
			byAgency[a.Name]++
			if r.ReceivedFrom != a.Name || r.Origin.Channel != a.Kind {
				t.Errorf("%s sold %s as %q in %q", a.Name, r.RecordLocator, r.ReceivedFrom, r.Origin.Channel)
			}
		case r.Origin.Agent == "wholesky direct" || r.Origin.Agent == "SKY00OTA":
		default:
			t.Errorf("record %s was sold by %q", r.RecordLocator, r.Origin.Agent)
		}
	}
	if len(byAgency) < 3 {
		t.Errorf("the agencies that sold through the distribution system: %v", byAgency)
	}
	// The ledger: the carriers' own sales are counted, and the agencies
	// have accounts with what the distribution system paid them.
	deadline := time.Now().Add(5 * time.Second)
	var direct, incentive int64
	var sold []string
	for time.Now().Before(deadline) {
		direct, incentive, sold = 0, 0, nil
		for code := range s.Tenants {
			acct := s.Ledger.AccountOf(code)
			direct += int64(acct.Channels[market.Direct] + acct.Channels["ndc"])
		}
		for _, a := range s.Agencies() {
			incentive += a.Incentive
			if a.Tickets > 0 {
				sold = append(sold, a.Name)
				if !distribution.Valid(a.Code) || a.Income != a.Incentive+a.Commission+a.Fees {
					t.Fatalf("agency %+v", a)
				}
			}
		}
		if direct > 0 && incentive > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	sort.Strings(sold)
	t.Logf("ledger: %d tickets on carriers' own systems; agencies with sales %v; incentives %d cents", direct, sold, incentive)
	if direct == 0 || incentive == 0 || len(sold) < 3 {
		t.Errorf("ledger: %d own tickets, %d in incentives, agencies %v", direct, incentive, sold)
	}
	rr := httptest.NewRecorder()
	s.serveAgencies(rr, httptest.NewRequest("GET", "/agencies.json", nil))
	var body struct {
		Agencies []agencyView `json:"agencies"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || len(body.Agencies) != len(s.commerce.agencies) || body.Agencies[0].Sales <= 0 {
		t.Errorf("/agencies.json: %v, %d agencies", err, len(body.Agencies))
	}
}

// A carrier that holds its last seats for its own site. With the lowest
// class closed on a flight, such a carrier tells the distribution
// systems that the class above it is closed too, sells that class on
// its own system, and a distribution system does not sell it. A carrier
// that treats every channel alike tells the systems what it knows. The
// lever turns the one into the other.
func TestACarrierHoldsItsLastSeatsForItsOwnSite(t *testing.T) {
	// The timed broadcast of every flight is 5 seconds apart, so that a
	// slow machine has time for the rest. A class that changes is told at
	// once and does not wait for it.
	s := bootWorld(t, Options{GDSCount: 1, AVSInterval: 5 * time.Second})
	ctx := context.Background()
	morning(s)
	w := seatWorld{s}
	var holds, alike string
	for code := range s.Tenants {
		if len(s.Flights[code]) == 0 {
			continue
		}
		if s.policyOf(code).DirectFirst && (holds == "" || code < holds) {
			holds = code
		}
		if !s.policyOf(code).DirectFirst && (alike == "" || code < alike) {
			alike = code
		}
	}
	if holds == "" || alike == "" {
		t.Skipf("the small world has no carrier of each kind: holds %q, alike %q", holds, alike)
	}
	wire := strings.ToUpper(s.BookingDate.Format("02Jan"))
	g := s.GDSes[0]
	status := func(code string, f worldFlight, class string) avail.Status {
		return g.GW.Avail.Status(avail.NewKey(code, f.Number, s.BookingDate, f.From, f.To, class))
	}
	told := func(code string, f worldFlight, class string, want avail.Status) bool {
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if status(code, f, class) == want {
				return true
			}
		}
		return false
	}
	lateFlight := func(code string) worldFlight {
		for _, f := range s.Flights[code] {
			if f.DepMin > 8*60 && f.Marketing == "" {
				return f
			}
		}
		return s.Flights[code][0]
	}
	sell := func(gw *gateway.Gateway, f worldFlight, class, name string) (string, error) {
		res, err := gw.Book(ctx, &gateway.BookingRequest{
			Passengers: []gateway.BookingPassenger{{Surname: name, Given: "ANN", Title: "MS"}},
			Segments:   []gateway.BookingSegment{{Carrier: f.Carrier, FlightNum: f.Number, Class: class, Date: wire, Board: f.From, Off: f.To, Seats: 1}},
			Agent:      "test", Channel: market.Direct, PricedAt: time.Now().AddDate(0, 0, -1),
		})
		if err != nil {
			return "", err
		}
		return res.PNR.Segments[0].Status, nil
	}

	// The carrier that holds seats: N closed, so S is closed to the
	// systems and open at home.
	f := lateFlight(holds)
	tn := s.Tenants[holds]
	// Its own site sells the lowest class until revenue management has
	// sold all it means to at that fare.
	sold := 0
	for ; sold < 80; sold++ {
		if st, err := sell(tn.Gateway, f, "N", "LOWEST"); err != nil || st != "HK" {
			break
		}
	}
	t.Logf("%s sold %d seats of %d on %s%s in N before the class closed", holds, sold, f.Seats, f.Carrier, f.Number)
	if sold == 0 || sold >= 80 {
		t.Fatalf("the lowest class closed after %d seats", sold)
	}
	if !told(holds, f, "N", avail.Closed) || !told(holds, f, "S", avail.Closed) {
		t.Fatalf("%s closed N on %s%s and told the systems N %s, S %s", holds, f.Carrier, f.Number, status(holds, f, "N"), status(holds, f, "S"))
	}
	if st := status(holds, f, "V"); st == avail.Closed {
		t.Errorf("V is two classes above the closed one and was told closed")
	}
	if open := s.trueOpen(tn, f.Carrier, f.Number, f); open["N"] || !open["S"] {
		t.Errorf("the carrier's own view: N open %v, S open %v", open["N"], open["S"])
	}
	if st, err := sell(tn.Gateway, f, "S", "ATHOME"); err != nil || st != "HK" {
		t.Errorf("the carrier's own site sold S at %s (%v)", st, err)
	}
	// A distribution system told a class is closed still asks, and the
	// carrier answers. It answers no: the request is worth the fare less
	// what the channel costs, which is the class under it, and that is
	// closed.
	res, err := g.GW.Book(ctx, &gateway.BookingRequest{
		Passengers: []gateway.BookingPassenger{{Surname: "ATAGENT", Given: "ANN", Title: "MS"}},
		Segments:   []gateway.BookingSegment{{Carrier: f.Carrier, FlightNum: f.Number, Class: "S", Date: wire, Board: f.From, Off: f.To, Seats: 1}},
		Agent:      "test", Channel: market.Direct, PricedAt: time.Now().AddDate(0, 0, -1),
	})
	if err != nil {
		t.Fatal(err)
	}
	answer := "HN"
	for deadline := time.Now().Add(30 * time.Second); answer == "HN" && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if rec, err := s.GDSStore.GetPNR(ctx, res.PNR.RecordLocator); err == nil {
			answer = rec.Segments[0].Status
		}
	}
	if answer == "HK" || answer == "HN" {
		t.Errorf("a distribution system asked for S and the carrier answered %s", answer)
		rec, _ := s.GDSStore.GetPNR(ctx, res.PNR.RecordLocator)
		evs, _ := s.GDSStore.Events(ctx, rec.ID)
		for _, e := range evs {
			t.Logf("   gds event %s: %s", e.Type, e.Detail)
		}
		t.Logf("   sold %v", tn.Inventory.SoldByClass(f.Carrier, f.Number, wire, f.From, "Y"))
		lad, _ := tn.Inventory.Network.LadderFares(f.Carrier, f.Number, wire, f.From, "Y")
		t.Logf("   ladder %+v", lad)
	}
	// The class above is worth the one the carrier still sells: yes.
	if st, _ := sell(g.GW, f, "V", "ATAGENT2"); st != "HK" && st != "HN" {
		t.Errorf("a distribution system asked for V and was answered %s at once", st)
	}

	// The carrier that treats every channel alike: N closed is N closed,
	// and S is as it was.
	f2 := lateFlight(alike)
	if _, err := w.Act(ctx, alike, airline.Action{Kind: "class", Flight: f2.Carrier + f2.Number, Board: f2.From, Class: "N", Status: "C"}); err != nil {
		// No seat is held: close it on the inventory as the lever would.
		s.Tenants[alike].Inventory.SetOverride(f2.Carrier, f2.Number, wire, f2.From, "N", "UC")
		s.Tenants[alike].Touch(f2.Number, f2.From)
	}
	if !told(alike, f2, "N", avail.Closed) {
		t.Fatalf("%s closed N and the systems hear %s", alike, status(alike, f2, "N"))
	}
	time.Sleep(1500 * time.Millisecond) // time for a change to S to be told, had there been one
	if st := status(alike, f2, "S"); st == avail.Closed {
		t.Errorf("%s treats every channel alike and told the systems S was closed", alike)
	}

	// The lever: it starts holding its seats, and the systems are told.
	if _, err := w.Act(ctx, alike, airline.Action{Kind: "channel", Choice: "directfirst", Count: 1}); err != nil {
		t.Fatal(err)
	}
	if pol := s.policyOf(alike); !pol.DirectFirst || pol.FullContent {
		t.Errorf("after the lever: %+v", pol)
	}
	// The policy changed and no seat did: the next broadcast tells the
	// systems of every flight again.
	if !told(alike, f2, "S", avail.Closed) {
		t.Errorf("%s now holds its seats and the systems still hear S %s", alike, status(alike, f2, "S"))
	}
	if _, err := w.Act(ctx, alike, airline.Action{Kind: "channel", Choice: "nonsense"}); err == nil {
		t.Error("a lever that does not exist was pulled")
	}
	if st := s.snapshot(); st.Policies[alike].DirectFirst != true {
		t.Error("the policy is not in the state")
	}
}
