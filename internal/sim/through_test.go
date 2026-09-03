package sim

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/world"
)

// Inter-airline through check-in, end to end over the wire: a passenger
// booked on two carriers is accepted at the first carrier's counter with an
// onward connection on the second; the first carrier's departure control
// asks the second's (DCQCKI through the switch), the second seats the
// passenger on its own flight and answers (DCRCKA), and the first records
// the seat on the connection. Both manifests then know.
func TestThroughCheckInCrossesCarriers(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()

	var f1, f2 world.Flight
	found := false
	for _, c := range s.Manifest.Carriers {
		for _, a := range s.Flights[c.Designator] {
			for _, b := range s.flightsByOrigin[a.To] {
				if b.Carrier != a.Carrier && b.DepMin >= a.ArrMin+40 && b.To != a.From {
					f1, f2, found = a, b, true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Skip("the trimmed world offers no cross-carrier connection")
	}
	date := strings.ToUpper(s.BookingDate.Format("02Jan"))
	res, err := s.GDS.Book(ctx, &gateway.BookingRequest{
		Passengers: []gateway.BookingPassenger{{Surname: "THROUGH", Given: "IATCI", Title: "MR"}},
		Segments: []gateway.BookingSegment{
			{Carrier: f1.Carrier, FlightNum: f1.Number, Class: "Y", Date: date, Board: f1.From, Off: f1.To, Seats: 1},
			{Carrier: f2.Carrier, FlightNum: f2.Number, Class: "Y", Date: date, Board: f2.From, Off: f2.To, Seats: 1},
		},
		Agent: "iatci", Channel: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	settleAll(t, s, []string{res.PNR.RecordLocator})
	quiesce(t, s)

	ta, tb := s.Tenants[f1.Carrier], s.Tenants[f2.Carrier]
	day := s.BookingDate
	if err := ta.SendPNL(ctx, f1, day); err != nil {
		t.Fatal(err)
	}
	if err := tb.SendPNL(ctx, f2, day); err != nil {
		t.Fatal(err)
	}
	key1 := dcs.Key{Flight: f1.Carrier + f1.Number, Date: date, Board: f1.From}
	key2 := dcs.Key{Flight: f2.Carrier + f2.Number, Date: date, Board: f2.From}
	waitFlight := func(tn *host.Tenant, key dcs.Key, cond func(*dcs.Flight) bool, what string) *dcs.Flight {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for {
			fl, err := tn.DCS.Flight(key)
			if err == nil && cond(fl) {
				return fl
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s never happened on %s", what, key.Flight)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	hasName := func(fl *dcs.Flight) bool {
		for _, p := range fl.Passengers {
			if p.Surname == "THROUGH" {
				return true
			}
		}
		return false
	}
	waitFlight(ta, key1, hasName, "the name list reaching the first airport")
	waitFlight(tb, key2, hasName, "the name list reaching the second airport")

	// The first carrier's counter accepts THROUGH with the onward leg on the
	// other carrier, which is what makes it a through check-in.
	acc, err := ta.DCS.Accept(ctx, key1, dcs.AcceptRequest{Surname: "THROUGH", Bags: []int{18},
		Onward: &dcs.Connection{Flight: f2.Carrier + f2.Number, Date: date, Station: f1.To, Dest: f2.To, Class: "Y", Carrier: f2.Carrier}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ta.ThroughCheck(ctx, f1, day, key1, acc.Flight, acc.Passengers); err != nil {
		t.Fatal(err)
	}

	// The second carrier's DCS accepted the passenger from the wire...
	fl2 := waitFlight(tb, key2, func(fl *dcs.Flight) bool {
		for _, p := range fl.Passengers {
			if p.Surname == "THROUGH" && p.Status == dcs.StatusAccepted {
				return true
			}
		}
		return false
	}, "the second carrier accepting the through-checked passenger")
	var onB *dcs.Passenger
	for _, p := range fl2.Passengers {
		if p.Surname == "THROUGH" {
			onB = p
		}
	}
	if onB.Seat == "" || onB.Sequence == 0 || onB.Inbound == nil || onB.Inbound.Flight != f1.Carrier+f1.Number || len(onB.Bags) != 1 {
		t.Fatalf("accepted on the second carrier with a seat, a sequence, the inbound flight and the connecting bag: %+v", onB)
	}
	// ...and the first carrier heard the answer and wrote the seat on the
	// connection.
	fl1 := waitFlight(ta, key1, func(fl *dcs.Flight) bool {
		for _, p := range fl.Passengers {
			if p.Surname == "THROUGH" && p.Onward != nil && p.Onward.Seat != "" {
				return true
			}
		}
		return false
	}, "the first carrier recording the onward seat")
	for _, p := range fl1.Passengers {
		if p.Surname == "THROUGH" && p.Onward.Seat != onB.Seat {
			t.Fatalf("the seat on the connection is the seat the other carrier gave: %s vs %s", p.Onward.Seat, onB.Seat)
		}
	}
}
