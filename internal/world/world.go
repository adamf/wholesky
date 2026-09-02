// Package world compiles a planet.
//
// Input: the OpenFlights snapshot vendored under data/ -- airports with
// coordinates and timezones, airlines, and the route graph -- plus a seed and
// a scale. Output: a deterministic Manifest naming every carrier, airport and
// scheduled flight the simulation will run. The same inputs always produce the
// same world, which is what makes a run repeatable and a bug in an emergent
// behaviour bisectable.
//
// The data is deliberately allowed to be stale. The simulation needs a
// plausible planet, not this year's planet; anyone holding a current OAG or
// Cirium snapshot can feed it through the same compiler and get today's world
// instead. Attribution: the vendored snapshot is the OpenFlights database
// (openflights.org), Open Database License.
package world

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Airport is a place a flight can touch.
type Airport struct {
	IATA string `json:"iata"`
	// ICAO is the four-letter location indicator air traffic services use;
	// empty for a field the snapshot has none for.
	ICAO    string  `json:"icao,omitempty"`
	Name    string  `json:"name"`
	City    string  `json:"city"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	TZ      string  `json:"tz"` // IANA name, e.g. "Europe/London"
}

// Carrier is one airline the world runs.
type Carrier struct {
	// Designator is the two-character IATA code, unique within a compiled
	// world -- the compiler drops the smaller twin of a collision, because a
	// Type B address has room for exactly two characters of carrier.
	Designator string `json:"designator"`
	Name       string `json:"name"`
	Country    string `json:"country"`
	// ICAO is the carrier's three-letter designator -- BAW, DLH, AAL -- which
	// heads its callsigns and its AFTN addresses. Synthesised when the
	// snapshot lacks one.
	ICAO string `json:"icao,omitempty"`
	// Hub is the airport this carrier touches most, which anchors its
	// teletype address and its aircraft rotations.
	Hub string `json:"hub"`
	// TTYAddress is the carrier's reservations address: hub city, "RM", the
	// designator. Synthetic but shaped like the real thing.
	TTYAddress string `json:"tty_address"`
	// Format is the wire dialect this carrier speaks: "typeb" or "edifact".
	// Assigned deterministically so roughly the real-world share of each is
	// represented and a given world always assigns the same way.
	Format string `json:"format"`
	// Transport is how the carrier's circuit reaches the switch: "tcp" for
	// the plain framed link, "matip" for the airline transport (RFC 2351),
	// which carries Type B only. Assigned by the same stable hash as Format.
	Transport string `json:"transport,omitempty"`
	// Routes is how many directed route entries the carrier operates, kept
	// because it is the size measure sigma filters on.
	Routes int `json:"routes"`
}

// Flight is one scheduled leg, daily unless Days says otherwise.
type Flight struct {
	Carrier string `json:"carrier"`
	Number  string `json:"number"` // four digits, zero-padded
	From    string `json:"from"`
	To      string `json:"to"`
	// DepMin and ArrMin are minutes after 0000 UTC. ArrMin may exceed 1440,
	// meaning arrival the next calendar day.
	DepMin int `json:"dep_min"`
	ArrMin int `json:"arr_min"`
	// BlockMin is the scheduled block time in minutes.
	BlockMin int `json:"block_min"`
	// Equipment is a coarse type label; Seats is the cabin it offers.
	Equipment string `json:"equipment"`
	Seats     int    `json:"seats"`
	// KM is the great-circle distance, kept because demand and delay models
	// both want it.
	KM int `json:"km"`
	// Tail is the registration of the aircraft rotation this leg belongs to.
	// The compiler chains each carrier's flights into rotations -- a tail
	// departs where it last arrived, after a turnaround -- so the movement
	// stream is coherent: the same registration works its way around the
	// network instead of teleporting. On a replay it is the real one.
	Tail string `json:"tail,omitempty"`
	// Marketing and MarketingNumber name the codeshare the flight was sold
	// as, when that differs from who flew it: AS3000 operated by OO.
	Marketing       string `json:"marketing,omitempty"`
	MarketingNumber string `json:"marketing_number,omitempty"`
	// Actual is what really happened, on a replayed day. Nil on a synthetic
	// one, whose delays the flight day invents.
	Actual *Actual `json:"actual,omitempty"`
}

// Manifest is a compiled world.
type Manifest struct {
	// Seed and Scale record how this world was made, so it can be made again.
	Seed  int64   `json:"seed"`
	Scale float64 `json:"scale"` // sigma: 1.0 is the whole sky
	// Region is the continent filter the compiler applied, empty for all.
	Region string `json:"region,omitempty"`

	Airports []Airport `json:"airports"`
	Carriers []Carrier `json:"carriers"`
	Flights  []Flight  `json:"flights"`
	// Replay is set when the manifest is a recording of a real day.
	Replay *Replay `json:"replay,omitempty"`

	GeneratedAt time.Time `json:"generated_at"`
}

// ByIATA indexes the manifest's airports.
func (m *Manifest) ByIATA() map[string]*Airport {
	out := make(map[string]*Airport, len(m.Airports))
	for i := range m.Airports {
		out[m.Airports[i].IATA] = &m.Airports[i]
	}
	return out
}

// CarrierByCode indexes the manifest's carriers.
func (m *Manifest) CarrierByCode() map[string]*Carrier {
	out := make(map[string]*Carrier, len(m.Carriers))
	for i := range m.Carriers {
		out[m.Carriers[i].Designator] = &m.Carriers[i]
	}
	return out
}

// Stats summarises a world in one line, the way a compile step should.
func (m *Manifest) Stats() string {
	seats := 0
	for _, f := range m.Flights {
		seats += f.Seats
	}
	return fmt.Sprintf("%d airports, %d carriers, %d flights/day, %d seats/day",
		len(m.Airports), len(m.Carriers), len(m.Flights), seats)
}

// haversineKM is the great-circle distance between two points.
func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp := (lat2 - lat1) * math.Pi / 180
	dl := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

// equipmentFor picks a coarse aircraft type and cabin size by stage length.
//
// The bands are deliberately crude -- the point is that a Helsinki puddle-jump
// does not fly on a 777 and a transpacific leg does not fly on a turboprop,
// so capacities and block times land in the right order of magnitude.
func equipmentFor(km float64) (string, int) {
	switch {
	case km < 500:
		return "AT7", 70
	case km < 1500:
		return "320", 180
	case km < 4000:
		return "321", 220
	case km < 8000:
		return "789", 290
	default:
		return "77W", 360
	}
}

// blockMinutes estimates scheduled block time from distance.
//
// Cruise plus a fixed taxi-and-climb allowance; padded a little on short legs
// the way real schedules are.
func blockMinutes(km float64) int {
	cruise := km / 820 * 60 // km/h to minutes
	pad := 40.0
	if km < 800 {
		pad = 30
	}
	return int(math.Round(cruise + pad))
}

// icaoDesignator gives a carrier a three-letter ICAO designator: the one the
// snapshot has, or one made from its IATA code so callsigns and AFTN
// addresses have something to carry. A made-up one is not any real
// carrier's; the digits real IATA codes contain map onto letters.
func icaoDesignator(iata, icao string) string {
	if len(icao) == 3 && icao != "N/A" {
		return icao
	}
	var b []byte
	for _, r := range iata {
		switch {
		case r >= 'A' && r <= 'Z':
			b = append(b, byte(r))
		case r >= '0' && r <= '9':
			b = append(b, byte('Q'+(r-'0')%10))
		}
	}
	for len(b) < 3 {
		b = append(b, 'X')
	}
	return string(b[:3])
}

// sortCarriers orders carriers largest first, then by code for determinism.
func sortCarriers(cs []Carrier) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Routes != cs[j].Routes {
			return cs[i].Routes > cs[j].Routes
		}
		return cs[i].Designator < cs[j].Designator
	})
}
