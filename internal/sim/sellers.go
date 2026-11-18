package sim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/pnr"

	"github.com/adamf/wholesky/internal/distribution"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/market"
	"github.com/adamf/wholesky/internal/revenue"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// Who sells the tickets.
//
// A traveller buys from the carrier or from an agency. The carrier's own
// site is the carrier's own system: the booking is made on the carrier's
// node, the seat comes out of its inventory with nothing on a wire, and
// no distribution system is paid. An agency is a named seller of the
// world (internal/distribution): it books through its distribution
// system, which sends the sell to the carrier and pays the agency an
// incentive, or over a direct connection to the carrier's node where
// both sides have one. The record says who sold it, the ledger keeps an
// account for each agency, and a carrier that holds seats for its own
// channels can tell which requests are its own.

// initSellers gives the world its agencies.
func (c *commerce) initSellers() {
	var systems []string
	for _, g := range gdsSlots {
		systems = append(systems, g.Designator)
	}
	c.agencies = distribution.Agencies(systems)
	c.agency = map[string]distribution.Agency{}
	for _, a := range c.agencies {
		c.agency[a.Code] = a
	}
}

// channelCost is what a sale through a distribution system costs a
// carrier over one on its own system, for each segment, while the
// carrier holds seats for its own channels. See host.Options.ChannelCost.
func (s *Sim) channelCost(carrier string) (int64, bool) {
	pol := s.policyOf(carrier)
	if !pol.DirectFirst {
		return 0, false
	}
	region := ""
	if s.econ != nil {
		region = s.econ.region[s.carriers[carrier].Hub]
	}
	return distribution.SegmentFee(region, pol.FullContent, true), true
}

// ownNode is a carrier's own system when this machine runs it.
func (s *Sim) ownNode(carrier string) *host.Tenant {
	t := s.Tenants[carrier]
	if t == nil || s.External(carrier) {
		return nil
	}
	return t
}

// nodeOf is the distribution system an agency books through, when this
// machine runs it, and otherwise one of those it does run, always the
// same one for the same agency.
func (s *Sim) nodeOf(a distribution.Agency, fallback *GDSNode) *GDSNode {
	for _, g := range s.GDSes {
		if g.Designator == a.System {
			return g
		}
	}
	if len(s.GDSes) == 0 || a.Code == "" {
		return fallback
	}
	return s.GDSes[aogHash(a.Code)%len(s.GDSes)]
}

// trueOpen is the classes a carrier's own system will sell on one of
// its flights today, for the carriers that tell the distribution systems
// less: the carrier's own site reads its own inventory. The answer is
// kept for two seconds, which is many travellers.
func (s *Sim) trueOpen(t *host.Tenant, carrier, number string, f world.Flight) map[string]bool {
	key := carrier + number + "/" + f.From
	s.commerce.openMu.Lock()
	defer s.commerce.openMu.Unlock()
	if c, ok := s.commerce.open[key]; ok && time.Since(c.at) < 2*time.Second {
		return c.classes
	}
	var keys []avail.Key
	for _, cls := range tariff.Ladder() {
		keys = append(keys, avail.NewKey(carrier, number, s.BookingDate, f.From, f.To, cls))
	}
	classes := map[string]bool{}
	for _, e := range t.Inventory.Availability(keys, time.Now().UTC()) {
		classes[e.Key.Class] = e.Status != avail.Closed
	}
	if len(s.commerce.open) > 20000 {
		s.commerce.open = map[string]openClasses{}
	}
	s.commerce.open[key] = openClasses{at: time.Now(), classes: classes}
	return classes
}

type openClasses struct {
	at      time.Time
	classes map[string]bool
}

// atCarrier books a sale on the carrier's own system: its website's
// booking, at the fare the traveller was quoted. It reports the record's
// locator, or false when the carrier's own inventory would not sell it.
func (s *Sim) atCarrier(ctx context.Context, tn *host.Tenant, req *gateway.BookingRequest) (string, bool) {
	res, err := tn.Gateway.Book(ctx, req)
	if err != nil {
		s.noteDemandFailure(err)
		return "", false
	}
	for _, sg := range res.PNR.Segments {
		if sg.Type != pnr.SegmentAir {
			continue
		}
		if sg.Status != "HK" && sg.Status != "HL" {
			// The seat went while the traveller was choosing. They do not
			// keep a booking with no seat in it.
			tn.Gateway.Cancel(ctx, res.PNR.RecordLocator, gateway.CancelOptions{By: "demand", Reason: "not sold: " + sg.Status}) //nolint:errcheck
			s.DemRefused.Add(1)
			return "", false
		}
	}
	return res.PNR.RecordLocator, true
}

// orderAt places an NDC order on a node's HTTP surface: a carrier's own,
// for its site and for an agency connected to it directly, or a
// distribution system's.
func (s *Sim) orderAt(node, agencyName, agencyID, carrier, number, from, to string, depMin int, class, surname string) error {
	date := s.BookingDate.Format("2006-01-02")
	dep := fmt.Sprintf("%02d:%02d", (depMin/60)%24, depMin%60)
	body := fmt.Sprintf(`<OrderCreateRQ xmlns="http://www.iata.org/IATA/EDIST">
  <Document><MessageVersion>1.1</MessageVersion></Document>
  <Party><Sender><TravelAgencySender>
    <Name>%s</Name><AgencyID>%s</AgencyID>
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
</OrderCreateRQ>`, strings.ToUpper(agencyName), agencyID, surname, carrier, carrier, number, from, date, dep, to, carrier, number, class)

	s.consolesMu.Lock()
	h := s.consoles[strings.ToUpper(node)]
	s.consolesMu.Unlock()
	if h == nil {
		return fmt.Errorf("no console for %s here", node)
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

// agencyView is one agency with its account, for the instruments.
type agencyView struct {
	distribution.Agency
	revenue.Agent
	Income int64 `json:"income"`
}

// Agencies is the world's sellers and what each has sold and earned.
func (s *Sim) Agencies() []agencyView {
	if s.commerce == nil {
		return nil
	}
	s.revenueMu.RLock()
	sold := s.agentFeed
	s.revenueMu.RUnlock()
	if sold == nil {
		sold = s.Ledger.Agents()
	}
	out := make([]agencyView, 0, len(s.commerce.agencies))
	for _, a := range s.commerce.agencies {
		acct := sold[a.Code]
		out = append(out, agencyView{Agency: a, Agent: acct, Income: acct.Income()})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Sales > out[j].Sales })
	return out
}

// serveAgencies is GET /agencies.json.
func (s *Sim) serveAgencies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"agencies": s.Agencies(), //nolint:errcheck
		"note": "Money is in cents. The agencies are invented. Incentive is what the agency's distribution system paid it, commission what the carriers paid it, and fees what it charged travellers."})
}

// seller is who a filled record was sold by, for the fill: the agency of
// a kind for the flight's part of the world, the distribution system it
// booked through, and whether it used a direct connection to the
// carrier.
func (s *Sim) seller(rng *rand.Rand, kind, carrier string, f world.Flight) (agent, system string, connected bool) {
	region := ""
	if s.econ != nil {
		region = s.econ.region[f.From]
	}
	a, ok := distribution.Pick(rng, s.commerce.agencies, kind, region)
	if !ok {
		return "", "", false
	}
	connected = a.Connect && s.policyOf(carrier).Connect && rng.Float64() < a.ConnectShare
	return a.Code, a.System, connected
}

// evPolicy is a carrier's way of selling, changed: every machine prices
// and counts by it.
const evPolicy = "policy"

// setChannel changes how a carrier sells, on a seat's word.
//
//	leave, join        a channel: ota, tmc or agency
//	surcharge          a channel, by an amount in cents; 0 takes it off
//	override           a channel, by hundredths of a percent of the fare
//	webonly            1 keeps the lowest fares to the carrier's own site
//	directfirst        1 holds the last seats of each class for its own
//	                   channels, and gives up the full-content deal
//	fullcontent        1 takes the deal: no surcharge, no fares or seats
//	                   held back
//	connect            1 offers agencies a direct connection
func (s *Sim) setChannel(carrier, lever, channel string, amount int) (string, error) {
	lever, channel = strings.ToLower(lever), strings.ToLower(channel)
	needs := map[string]bool{"leave": true, "join": true, "surcharge": true, "override": true}
	if needs[lever] && channel != market.OTA && channel != market.TMC && channel != market.Agency {
		return "", fmt.Errorf("%s needs a channel: ota, tmc or agency", lever)
	}
	on := amount != 0
	var problem error
	pol := s.setPolicy(carrier, func(p *distribution.Policy) {
		switch lever {
		case "leave":
			delete(p.Sells, channel)
		case "join":
			p.Sells[channel] = true
		case "surcharge":
			if amount <= 0 {
				delete(p.Surcharge, channel)
			} else {
				p.Surcharge[channel] = int64(amount)
				p.FullContent = false
			}
		case "override":
			if amount <= 0 {
				delete(p.Override, channel)
			} else {
				p.Override[channel] = min(amount, 1000)
			}
		case "webonly":
			p.WebOnly = on
			if on {
				p.FullContent = false
			}
		case "directfirst":
			p.DirectFirst = on
			if on {
				p.FullContent = false
			}
		case "fullcontent":
			p.FullContent = on
			if on {
				p.WebOnly, p.DirectFirst, p.Surcharge = false, false, map[string]int64{}
			}
		case "connect":
			p.Connect = on
		default:
			problem = fmt.Errorf("no such channel lever %q: leave, join, surcharge, override, webonly, directfirst, fullcontent, connect", lever)
		}
	})
	if problem != nil {
		return "", problem
	}
	body, _ := json.Marshal(pol)
	s.publish(worldEvent{Kind: evPolicy, Carrier: strings.ToUpper(carrier), Body: body})
	s.saveState()
	b, _ := json.Marshal(pol)
	return "selling policy now " + string(b), nil
}
