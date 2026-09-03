package host

import (
	"strings"
	"testing"

	"github.com/adamf/jetway/pkg/baggage"
	"github.com/adamf/wholesky/internal/world"
)

// A short-shipped bag rides the carrier's next flight over the same sector
// today, and the message that goes ahead of it names that flight, the tag
// and the passenger.
func TestRushFlightIsTheNextOverTheSector(t *testing.T) {
	flights := []world.Flight{
		{Carrier: "BA", Number: "0117", From: "LHR", To: "JFK", DepMin: 8 * 60},
		{Carrier: "BA", Number: "0175", From: "LHR", To: "JFK", DepMin: 13 * 60},
		{Carrier: "BA", Number: "0115", From: "LHR", To: "JFK", DepMin: 10 * 60},
		{Carrier: "BA", Number: "0001", From: "LHR", To: "BOS", DepMin: 11 * 60},
		{Carrier: "AA", Number: "0100", From: "LHR", To: "JFK", DepMin: 9 * 60},
	}
	next, ok := rushFlightFor(flights, flights[0])
	if !ok || next.Number != "0115" {
		t.Errorf("next over LHR-JFK after 0800 is BA0115: %+v %v", next, ok)
	}
	if _, ok := rushFlightFor(flights, flights[1]); ok {
		t.Error("nothing later than 1300 over the sector today")
	}
	m := &baggage.Message{Kind: baggage.KindBUM, Version: "1LHR", Outbound: &baggage.FlightLeg{Flight: "BA0115", Date: "26NOV", City: "JFK"},
		Tags: []baggage.Tag{{Number: "0125123456", Count: 1}}, Surname: "SMITH", Elements: []string{".X/RUSH EX BA0117"}}
	text, err := baggage.Build(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"BUM\n", ".F/BA0115/26NOV/JFK", ".N/0125123456001", ".P/SMITH", ".X/RUSH EX BA0117", "ENDBUM"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
}
