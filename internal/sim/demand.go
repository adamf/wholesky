package sim

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/fare"
	"github.com/adamf/jetway/pkg/gateway"

	"github.com/adamf/wholesky/internal/world"
)

// Demand generates travellers continuously at roughly perMinute.
//
// This is the second demand model. The first booked one random segment and
// walked away, which left Jetway's richest machinery idle: nobody ticketed,
// so deadlines never armed; nobody cancelled; nobody split; no record ever
// spanned two carriers. This one books itineraries -- connections included,
// across carriers when the schedule offers them -- with parties of one to
// three, then lives with the booking: most get ticketed, some are cancelled,
// parties sometimes divide, and a slice of demand arrives as NDC orders over
// the GDS's own HTTP endpoint rather than through the booking API.
//
// Randomness is seeded; the mix is fixed. The point is not statistical
// fidelity to any airline's curve -- it is that every lifecycle path Jetway
// implements gets walked continuously, by load, in public.
func (s *Sim) Demand(ctx context.Context, perMinute int, seed int64) {
	if perMinute <= 0 {
		return
	}
	var carriers []string
	for code := range s.Flights {
		if len(s.Flights[code]) > 0 {
			carriers = append(carriers, code)
		}
	}
	if len(carriers) == 0 {
		return
	}
	for i := 1; i < len(carriers); i++ {
		for j := i; j > 0 && carriers[j] < carriers[j-1]; j-- {
			carriers[j], carriers[j-1] = carriers[j-1], carriers[j]
		}
	}

	interval := time.Minute / time.Duration(perMinute)
	tick := time.NewTicker(interval)
	defer tick.Stop()
	report := time.NewTicker(30 * time.Second)
	defer report.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-report.C:
			s.log.Info("demand",
				"booked", s.DemBooked.Load(), "failed", s.DemFailed.Load(),
				"interline", s.DemInterline.Load(), "ndc", s.DemNDC.Load(),
				"ticketed", s.DemTicketed.Load(), "cancelled", s.DemCancelled.Load(),
				"split", s.DemSplit.Load(), "movements", s.Movements.Load())
		case <-tick.C:
			n++
			// Each traveller gets a private generator seeded from the draw,
			// so the concurrent lifecycles do not contend for one rng.
			sub := rand.New(rand.NewSource(seed<<16 ^ int64(n)))
			// Travellers spread across the distribution systems, the way real
			// demand arrives through whichever channel the agent uses.
			go s.placeDemand(ctx, s.GDSes[n%len(s.GDSes)], sub, carriers, n)
		}
	}
}

// sellOffset picks which day a traveller books for. The default is the day
// the world flies: a simulated day is minutes of wall time, so bookings
// spread over four dates gave the flown one a quarter of a volume that was
// already a sixtieth of real -- and every flight left nearly empty. A
// deployment that wants the selling window back sets SellDays above one.
func (s *Sim) sellOffset(rng *rand.Rand) int {
	days := s.sellDays
	if days <= 1 {
		return 0
	}
	return rng.Intn(days) - 1
}

// placeDemand runs one traveller end to end: choose, book, settle, and live
// with the consequences.
func (s *Sim) placeDemand(ctx context.Context, g *GDSNode, rng *rand.Rand, carriers []string, n int) {
	itin := s.chooseItinerary(rng, carriers)
	if len(itin) == 0 {
		return
	}
	party := 1 + rng.Intn(3)
	day := s.sellOffset(rng)
	// Booked on the day: the cheap fares' advance-purchase rules have
	// closed, so most of this demand is full fare, some business, a little
	// first. A class the tariff will not sell for the trip falls back to Y.
	class := []string{"Y", "Y", "Y", "Y", "Y", "M", "B", "J", "F"}[rng.Intn(9)]
	surname := fmt.Sprintf("DEMAND%06d", n)

	interline := len(itin) == 2 && itin[0].Carrier != itin[1].Carrier

	// A slice of single-segment demand arrives as an NDC order on the GDS's
	// HTTP endpoint -- the distribution channel, not just the message.
	if len(itin) == 1 && rng.Intn(100) < 18 {
		if err := s.bookNDC(g, itin[0].Carrier, itin[0].Number, itin[0].From, itin[0].To,
			itin[0].DepMin, class, day, surname); err != nil {
			s.DemFailed.Add(1)
			return
		}
		s.DemNDC.Add(1)
		s.DemBooked.Add(1)
		return
	}

	segs := make([]gateway.BookingSegment, 0, len(itin))
	date := strings.ToUpper(s.BookingDate.AddDate(0, 0, day).Format("02Jan"))
	for _, f := range itin {
		carrier, number := f.Carrier, f.Number
		// A codeshare sells under the marketing carrier's code nine times in
		// ten; the operator's own code is the exception.
		if f.Marketing != "" && f.Marketing != f.Carrier && f.MarketingNumber != "" && rng.Intn(10) < 9 {
			carrier, number = f.Marketing, f.MarketingNumber
		}
		segs = append(segs, gateway.BookingSegment{
			Carrier: carrier, FlightNum: number, Class: class,
			Date: date, Board: f.From, Off: f.To, Seats: party,
		})
	}
	pax := make([]gateway.BookingPassenger, 0, party)
	for p := 0; p < party; p++ {
		pax = append(pax, gateway.BookingPassenger{
			Surname: surname, Given: fmt.Sprintf("PAX%d", p+1), Title: "MR",
		})
	}
	// A few travellers in every hundred need something at the airport --
	// a wheelchair, an unaccompanied child, a medical case -- and the
	// request rides the booking so the name list can carry it to check-in
	// and the PSM can carry it to the arrival station.
	var ssrs []gateway.BookingSSR
	if r := rng.Intn(100); r < 3 {
		codes := []string{"WCHR", "WCHR", "UMNR", "MEDA", "BLND", "DEAF", "MAAS"}
		ssrs = append(ssrs, gateway.BookingSSR{Code: codes[rng.Intn(len(codes))], Carrier: itin[0].Carrier})
	}
	res, err := g.GW.Book(ctx, &gateway.BookingRequest{
		Passengers: pax, Segments: segs, SSRs: ssrs, Agent: "wholesky", Channel: "sim",
	})
	var nf *fare.ErrNoFare
	if errors.As(err, &nf) {
		// The class cannot be sold for this trip under its rules; the
		// passenger pays the fare that can.
		for k := range segs {
			segs[k].Class = "Y"
		}
		res, err = g.GW.Book(ctx, &gateway.BookingRequest{
			Passengers: pax, Segments: segs, SSRs: ssrs, Agent: "wholesky", Channel: "sim",
		})
	}
	if err != nil {
		s.DemFailed.Add(1)
		return
	}
	s.DemBooked.Add(1)
	if interline {
		s.DemInterline.Add(1)
	}
	s.liveWith(ctx, g, rng, res.PNR.RecordLocator, party)
}

// liveWith is the rest of a booking's life: settle, then usually ticket,
// sometimes cancel, occasionally divide.
func (s *Sim) liveWith(ctx context.Context, g *GDSNode, rng *rand.Rand, locator string, party int) {
	deadline := time.Now().Add(20 * time.Second)
	for {
		ok, err := settledIn(ctx, g.Store, locator)
		if err != nil || time.Now().After(deadline) {
			return
		}
		if ok {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(150 * time.Millisecond):
		}
	}

	roll := rng.Intn(100)
	switch {
	case roll < 8:
		if _, err := g.GW.Cancel(ctx, locator, gateway.CancelOptions{
			By: "demand", Reason: "traveller cancelled",
		}); err == nil {
			s.DemCancelled.Add(1)
			s.demMu.Lock()
			s.DemCancelledLocs = append(s.DemCancelledLocs, locator)
			s.demMu.Unlock()
		}
		return
	case roll < 63:
		rec, err := g.Store.GetPNR(ctx, locator)
		if err != nil || len(rec.Segments) == 0 {
			return
		}
		code := s.accountingCode(rec.Segments[0].Carrier)
		if _, err := g.GW.IssueTickets(ctx, locator, gateway.IssueOptions{
			AirlineCode: code, IssuedBy: "demand",
		}); err == nil {
			s.DemTicketed.Add(1)
		}
	}

	// A party sometimes divides -- somebody's plans changed -- which is the
	// operation whose advisories and divergences the map draws.
	if party >= 2 && rng.Intn(100) < 12 {
		if _, err := g.GW.Split(ctx, gateway.SplitRequest{
			Locator: locator, Passengers: []int{party},
			By: "demand", Reason: "traveller separated",
		}); err == nil {
			s.DemSplit.Add(1)
		}
	}
}

// chooseItinerary picks one or two legs from the schedule: mostly single
// segments, a real share of connections, and among connections a real share
// that changes carrier -- the case interline messaging exists for.
func (s *Sim) chooseItinerary(rng *rand.Rand, carriers []string) []worldFlight {
	code := carriers[rng.Intn(len(carriers))]
	fs := s.Flights[code]
	f1 := fs[rng.Intn(len(fs))]
	if s.isClosed(f1.From, f1.To) {
		return nil
	}
	roll := rng.Intn(100)
	if roll < 60 || f1.ArrMin >= 23*60 {
		return []worldFlight{f1}
	}
	wantInterline := roll < 90 // of the connecting share, most change carrier
	onward := s.flightsByOrigin[f1.To]
	// A bounded scan for a legal connection: departing after arrival plus
	// minimum connect time, same day, airport open.
	start := rng.Intn(maxInt(1, len(onward)))
	for i := 0; i < len(onward) && i < 80; i++ {
		f2 := onward[(start+i)%len(onward)]
		if f2.DepMin < f1.ArrMin+40 || f2.To == f1.From {
			continue
		}
		if s.isClosed(f2.From, f2.To) {
			continue
		}
		if wantInterline == (f2.Carrier != f1.Carrier) {
			return []worldFlight{f1, f2}
		}
	}
	return []worldFlight{f1}
}

// accountingCode synthesises the three-digit numeric stock code a ticket is
// issued against, stable per carrier.
func (s *Sim) accountingCode(carrier string) string {
	h := 0
	for _, r := range carrier {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return fmt.Sprintf("%03d", 100+h%900)
}

// bookNDC places an order through the GDS's NDC endpoint: the same HTTP
// surface an OTA would call, XML and all.
func (s *Sim) bookNDC(g *GDSNode, carrier, number, from, to string, depMin int, class string, day int, surname string) error {
	date := s.BookingDate.AddDate(0, 0, day).Format("2006-01-02")
	dep := fmt.Sprintf("%02d:%02d", (depMin/60)%24, depMin%60)
	body := fmt.Sprintf(`<OrderCreateRQ xmlns="http://www.iata.org/IATA/EDIST">
  <Document><MessageVersion>1.1</MessageVersion></Document>
  <Party><Sender><TravelAgencySender>
    <Name>WHOLESKY OTA</Name><AgencyID>SKY00OTA</AgencyID>
  </TravelAgencySender></Sender></Party>
  <Query>
    <Passengers><Passenger ObjectKey="T1">
      <PTC>ADT</PTC>
      <Name><Surname>%s</Surname><Given>NDC</Given><Title>MR</Title></Name>
    </Passenger></Passengers>
    <OrderItems><OfferItem>
      <OfferItemID Owner="%s">1</OfferItemID>
      <OfferItemType><DetailedFlightItem refs="T1">
        <OriginDestination><Flight>
          <SegmentKey>%s%s</SegmentKey>
          <Departure><AirportCode>%s</AirportCode><Date>%s</Date><Time>%s</Time></Departure>
          <Arrival><AirportCode>%s</AirportCode></Arrival>
          <MarketingCarrier><AirlineID>%s</AirlineID><FlightNumber>%s</FlightNumber></MarketingCarrier>
          <ClassOfService><Code>%s</Code></ClassOfService>
        </Flight></OriginDestination>
      </DetailedFlightItem></OfferItemType>
    </OfferItem></OrderItems>
  </Query>
</OrderCreateRQ>`, surname, carrier, carrier, number, from, date, dep, to, carrier, number, class)

	s.consolesMu.Lock()
	h := s.consoles[g.Designator]
	s.consolesMu.Unlock()
	if h == nil {
		return fmt.Errorf("no gds console handler")
	}
	req := httptest.NewRequest("POST", "/ndc", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/xml")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code >= 300 {
		return fmt.Errorf("ndc order refused: %d %s", rec.Code, rec.Body.String())
	}
	return nil
}

type worldFlight = world.Flight

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
