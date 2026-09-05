package sim

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/world"
)

// Two worlds join: each with its own carriers, distribution systems and
// switch, the second told at boot where the first is. The handshake trunks
// the switches, routes each world's subscribers down the trunk, and hands
// each world the other's flights to sell. A seat sold by the second world's
// distribution system on the first world's carrier crosses the trunk and
// lands in that carrier's book.
func TestTwoWorldsJoinAndSellEachOthersFlights(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mA := smallWorld(t)
	mB, err := world.Compile(world.CompileOptions{DataDir: "../../data", Seed: 2, Countries: []string{"Spain", "Italy", "Netherlands"}, MaxCarriers: 3})
	if err != nil {
		t.Fatal(err)
	}
	// A designator is an address: the worlds' carriers must not overlap.
	inA := map[string]bool{}
	for _, c := range mA.Carriers {
		inA[c.Designator] = true
	}
	var keep []world.Carrier
	for _, c := range mB.Carriers {
		if !inA[c.Designator] {
			keep = append(keep, c)
		}
	}
	mB.Carriers = keep
	var flights []world.Flight
	for _, f := range mB.Flights {
		for _, c := range keep {
			if f.Carrier == c.Designator {
				flights = append(flights, f)
			}
		}
	}
	mB.Flights = flights
	if len(mB.Carriers) == 0 || len(mB.Flights) == 0 {
		t.Skip("no disjoint second world in the vendored data")
	}

	aAddr, bAddr := freeAddr(t), freeAddr(t)
	a, err := Boot(ctx, mA, Options{Log: log, Console: aAddr, WorldName: "alpha", PublicURL: "http://" + aAddr})
	if err != nil {
		t.Fatalf("world alpha: %v", err)
	}
	defer a.Stop()
	b, err := Boot(ctx, mB, Options{Log: log, Console: bAddr, WorldName: "bravo", WorldCode: "1Z", WorldCity: "MAD",
		PublicURL: "http://" + bAddr, PeerWorlds: []string{"http://" + aAddr}})
	if err != nil {
		t.Fatalf("world bravo: %v", err)
	}
	defer b.Stop()

	// The trunk comes up on both sides, named for the other world's switch.
	deadline := time.Now().Add(60 * time.Second)
	for {
		aSees, bSees := strings.Join(a.Switch.LivePeers(), ","), strings.Join(b.Switch.LivePeers(), ",")
		live := func(list, code string) bool {
			for _, p := range strings.Split(list, ",") {
				if p == code {
					return true
				}
			}
			return false
		}
		if live(aSees, "1Z") && live(bSees, "1X") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the worlds never trunked: alpha sees %s; bravo sees %s", aSees, bSees)
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Each world knows the other's carriers and flights.
	cA := mA.Carriers[1].Designator // the EDIFACT carrier of the small world
	cB := mB.Carriers[0].Designator
	deadline = time.Now().Add(30 * time.Second)
	for {
		_, okA := b.foreignCarrier(cA)
		_, okB := a.foreignCarrier(cB)
		if okA && okB && len(b.foreignFlights(cA)) > 0 && len(a.foreignFlights(cB)) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the worlds never learned each other's flights: bravo knows %s %v, alpha knows %s %v", cA, okA, cB, okB)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(a.Worlds()) != 1 || len(b.Worlds()) != 1 {
		t.Errorf("worlds: alpha %v bravo %v", a.Worlds(), b.Worlds())
	}

	// Bravo's distribution system sells a seat on alpha's carrier; the sell
	// crosses the trunk, alpha's tenant books it, the reply crosses back.
	fA := b.foreignFlights(cA)[0]
	res, err := b.Book(ctx, fA, "Y", 0, "TWOWORLDS")
	if err != nil {
		t.Fatalf("book across worlds: %v", err)
	}
	deadline = time.Now().Add(60 * time.Second)
	for {
		recs, _ := a.Tenants[cA].Store.ListPNRs(ctx, 1000)
		found := false
		for _, r := range recs {
			for _, p := range r.Passengers {
				if strings.EqualFold(p.Surname, "TWOWORLDS") {
					found = true
				}
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the booking %s never reached alpha's %s", res.PNR.RecordLocator, cA)
		}
		time.Sleep(100 * time.Millisecond)
	}
	// And the other way: alpha sells bravo's carrier.
	fB := a.foreignFlights(cB)[0]
	res, err = a.Book(ctx, fB, "Y", 0, "BACKAGAIN")
	if err != nil {
		t.Fatalf("book back across worlds: %v", err)
	}
	deadline = time.Now().Add(60 * time.Second)
	for {
		recs, _ := b.Tenants[cB].Store.ListPNRs(ctx, 1000)
		found := false
		for _, r := range recs {
			for _, p := range r.Passengers {
				if strings.EqualFold(p.Surname, "BACKAGAIN") {
					found = true
				}
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the booking %s never reached bravo's %s", res.PNR.RecordLocator, cB)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
