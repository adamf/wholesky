// Package market is the people who buy the tickets: who a traveller is,
// what they will pay, what a fare's conditions cost them, and which of
// the offers in front of them they take.
//
// The model is the passenger choice model of the Passenger Origin
// Destination Simulator, the simulator Boeing built and MIT has run
// since 1994 to test revenue management systems against each other, as
// Peter Belobaba's lectures for MIT course 16.75J describe it. It is
// used here because it is the one published model with its constants
// that has been shown to make competing airlines' systems behave like
// the real ones. A traveller is a business or a leisure traveller. Each
// has a most they will pay, drawn so that half of leisure travellers
// will pay 1.2 times the lowest fare and half of business travellers
// three times a base two and a half times higher. Each puts a price on
// a fare's restrictions: the Saturday night away, the money that cannot
// be had back, the fee to change. Each puts a price on a connection, on
// leaving at another hour than the one wanted, and on flying an airline
// that is not theirs. A traveller throws out the offers that cost more
// than they will pay and takes the one whose fare plus all those prices
// is least. One airline's closed class is then another airline's
// booking, and a fare cut wins the travellers it was aimed at.
//
// The constants PODS publishes are used as published. The ones it does
// not publish -- the booking curves by day, the cost of an hour, how
// much loyalty is worth, who books where -- are estimates and say so.
package market

import (
	"math"
	"math/rand"
)

// Kind is the kind of traveller.
type Kind int

const (
	Leisure Kind = iota
	Business
)

func (k Kind) String() string {
	if k == Business {
		return "business"
	}
	return "leisure"
}

// The channels a traveller books through.
const (
	Direct = "direct" // the carrier's own site, app or call centre
	OTA    = "ota"    // an online travel agency
	TMC    = "tmc"    // a travel management company, for an employer
	Agency = "agency" // a travel agent
)

// Traveller is one person, or one party, wanting to make one trip.
type Traveller struct {
	Kind  Kind
	Party int
	// Lead is how many days before departure they buy.
	Lead int
	// RoundTrip says they come back, after Stay days; SaturdayNight says
	// the stay they want holds one already.
	RoundTrip     bool
	Stay          int
	SaturdayNight bool
	// Channel is where they shop; Account the contract their employer
	// has, when it has one.
	Channel string
	Account string
	// Cabin is the cabin they will fly in where there is one: "" for
	// economy, "C" for business, "F" for first.
	Cabin string
	// Most is the most they will pay for one direction, in cents.
	Most int64
	// What a restriction costs them, in cents: a Saturday night they did
	// not want, a fare they cannot get back, a fee to change.
	SaturdayCost, NonRefundableCost, ChangeCost int64
	// What the journey costs them beyond the fare: a connection, each
	// hour off the departure time they wanted, booking outside their own
	// channel.
	ConnectionCost, HourCost, SwitchCost int64
	// Preferred is the departure they want, in minutes of the day.
	Preferred int
	// Loyal is the carrier whose programme they belong to, and
	// LoyaltyValue what flying it is worth to them; a partner of that
	// carrier is worth half.
	Loyal        string
	LoyaltyValue int64
}

// The published PODS constants.
const (
	// A leisure traveller's willingness to pay starts at the lowest fare
	// and half will pay 1.2 times it; a business traveller's starts at
	// 2.5 times the lowest fare and half will pay three times that.
	leisureMult  = 1.2
	businessBase = 2.5
	businessMult = 3.0
	// The mean cost of each restriction as a multiple of the lowest fare,
	// from the PODS example structure with a lowest fare of 200 dollars:
	// Saturday night 450 business and 350 leisure, non-refundable 150 and
	// 50, change fee 150 and 50.
	satBusiness, satLeisure       = 2.25, 1.75
	nonrefBusiness, nonrefLeisure = 0.75, 0.25
	changeBusiness, changeLeisure = 0.75, 0.25
	// A connection is a path quality index of 3 against a nonstop's 1.
	pathSteps = 2
)

// Estimates: what PODS has and does not print.
const (
	// The unit cost of path quality as a share of the lowest fare.
	pathBusiness, pathLeisure = 0.30, 0.10
	// An hour off the wanted departure, as a share of the lowest fare.
	hourBusiness, hourLeisure = 0.08, 0.03
	// The spread of each restriction's cost around its mean.
	spread = 0.30
	// Booking outside one's own channel: a traveller whose company books
	// through a travel management company pays in trouble to go round it
	// (moving a booking off the distribution system cost the large
	// agencies about fifteen dollars); a leisure traveller hardly at all.
	switchBusiness, switchLeisure = 2000, 300
)

// leisureCurve and businessCurve are the share of each kind's bookings
// made by each number of days before departure: the PODS time frames,
// with the shares read from its booking-curve chart and shaped to "about
// half of business demand inside two weeks". Estimates.
var curveDays = []int{63, 56, 49, 42, 35, 31, 28, 24, 21, 17, 14, 10, 7, 5, 3, 1, 0}
var leisureCurve = []float64{.08, .15, .22, .30, .39, .46, .53, .61, .68, .76, .83, .90, .94, .97, .99, 1, 1}
var businessCurve = []float64{.01, .03, .05, .08, .12, .16, .20, .26, .32, .41, .50, .63, .75, .85, .93, .98, 1}

// lead draws the days before departure a traveller buys.
func lead(rng *rand.Rand, k Kind) int {
	curve := leisureCurve
	if k == Business {
		curve = businessCurve
	}
	// curve[i] is the share who have booked by curveDays[i] days out, so
	// a draw between curve[i-1] and curve[i] is a traveller who books from
	// curveDays[i] days out up to the day before curveDays[i-1].
	u := rng.Float64()
	for i, c := range curve {
		if u > c {
			continue
		}
		lo := curveDays[i]
		if i == 0 {
			// The earliest bookers: nine weeks to five months out.
			return lo + rng.Intn(90)
		}
		hi := curveDays[i-1] - 1
		if hi <= lo {
			return lo
		}
		return lo + rng.Intn(hi-lo+1)
	}
	return 0
}

// Mix says who a market's travellers are and where they book.
type Mix struct {
	// Business is the share of travellers on business: about a third on
	// a short business route, a fifth on long haul. An estimate.
	Business float64
	// Channels is where each kind books, as shares that add to one, in
	// the order Direct, OTA, TMC, Agency.
	Leisure, Corporate [4]float64
	// PremiumLeisure and PremiumBusiness are the share of each kind that
	// flies in a premium cabin where the aircraft has one, and First the
	// share of those who fly in first where there is a first.
	PremiumLeisure, PremiumBusiness, First float64
}

// MixFor is the travellers of a market by the part of the world it is
// in. Where travellers book is from carriers' reports and court
// findings: US network carriers sold 55 to 65 per cent direct in 2019 to
// 2024, and the indirect ticket carried about 1.5 times the fare, which
// is the business traveller booking through a travel management
// company. Elsewhere agencies hold more. Estimates around those.
func MixFor(region string, longHaul bool) Mix {
	m := Mix{Business: 0.30, Leisure: [4]float64{0.60, 0.30, 0.00, 0.10}, Corporate: [4]float64{0.35, 0.08, 0.52, 0.05}}
	// Who sits at the front. A short-haul aircraft of a network carrier
	// has about a tenth of its seats there and a long-haul one a sixth,
	// and the paying load in them runs under the cabin behind: the shares
	// here put about 7 and 13 per cent of travellers in them. Estimates.
	m.PremiumLeisure, m.PremiumBusiness, m.First = 0.02, 0.18, 0.10
	if longHaul {
		m.Business = 0.20
		m.PremiumLeisure, m.PremiumBusiness = 0.07, 0.38
	}
	switch region {
	case "North America", "Europe", "Oceania", "":
	default:
		m.Leisure = [4]float64{0.40, 0.30, 0.00, 0.30}
		m.Corporate = [4]float64{0.25, 0.10, 0.40, 0.25}
	}
	return m
}

var channels = [4]string{Direct, OTA, TMC, Agency}

// Draw makes a traveller for a market whose lowest regular fare is base
// cents one way.
func Draw(rng *rand.Rand, base int64, mix Mix) Traveller {
	t := Traveller{Kind: Leisure, Party: 1}
	if rng.Float64() < mix.Business {
		t.Kind = Business
	}
	b := float64(base)
	// The most they will pay: P(pay at least f) falls by half for each
	// (mult - 1) x base over the starting fare.
	start, step := b, (leisureMult-1)*b
	if t.Kind == Business {
		start, step = businessBase*b, (businessMult-1)*businessBase*b
	}
	t.Most = int64(start + step*(-math.Log2(1-rng.Float64())))
	cost := func(mean float64) int64 {
		v := mean * b * (1 + spread*rng.NormFloat64())
		if v < 0 {
			v = 0
		}
		return int64(v)
	}
	shares := mix.Leisure
	if t.Kind == Business {
		t.SaturdayCost, t.NonRefundableCost, t.ChangeCost = cost(satBusiness), cost(nonrefBusiness), cost(changeBusiness)
		t.ConnectionCost, t.HourCost, t.SwitchCost = int64(pathSteps*pathBusiness*b), int64(hourBusiness*b), switchBusiness
		shares = mix.Corporate
		// Business travel is day trips and short stays, home for the
		// weekend: one stay in five holds a Saturday night.
		t.RoundTrip = rng.Float64() < 0.80
		t.Stay = 1 + rng.Intn(4)
		t.SaturdayNight = rng.Float64() < 0.20
		// Morning and evening.
		t.Preferred = []int{7 * 60, 8 * 60, 17 * 60, 18 * 60}[rng.Intn(4)] + rng.Intn(60)
	} else {
		t.SaturdayCost, t.NonRefundableCost, t.ChangeCost = cost(satLeisure), cost(nonrefLeisure), cost(changeLeisure)
		t.ConnectionCost, t.HourCost, t.SwitchCost = int64(pathSteps*pathLeisure*b), int64(hourLeisure*b), switchLeisure
		t.RoundTrip = rng.Float64() < 0.70
		t.Stay = 3 + rng.Intn(12)
		t.SaturdayNight = rng.Float64() < 0.75
		t.Preferred = 6*60 + rng.Intn(14*60)
		// Holidays are taken together.
		switch r := rng.Float64(); {
		case r < 0.45:
			t.Party = 2
		case r < 0.60:
			t.Party = 3
		case r < 0.68:
			t.Party = 4
		}
	}
	if t.SaturdayNight && t.Stay < 2 {
		t.Stay = 2
	}
	premium := mix.PremiumLeisure
	if t.Kind == Business {
		premium = mix.PremiumBusiness
	}
	if rng.Float64() < premium {
		// The traveller at the front has the money for it: what they will
		// pay starts at four times the lowest fare, about where a business
		// cabin's discount fare is filed, and half will pay eight times.
		// An estimate: PODS has one cabin.
		t.Cabin = "C"
		floor := 4 * b
		if rng.Float64() < mix.First {
			t.Cabin, floor = "F", 7*b
		}
		if most := int64(floor + 4*b*(-math.Log2(1-rng.Float64()))); most > t.Most {
			t.Most = most
		}
	}
	t.Lead = lead(rng, t.Kind)
	u := rng.Float64()
	for i, s := range shares {
		if u < s || i == 3 {
			t.Channel = channels[i]
			break
		}
		u -= s
	}
	if t.Channel == TMC && rng.Float64() < 0.6 {
		t.Account = "CORP"
	}
	return t
}

// Offer is one way to make the trip at one price.
type Offer struct {
	// Carriers are the airlines that fly it, the first being the one that
	// sells it.
	Carriers []string
	// Connections is how many times the traveller changes aircraft.
	Connections int
	// Depart is the departure, in minutes of the day.
	Depart int
	// Price is what one traveller pays for one direction, in cents, taxes
	// and the carrier's own fees in.
	Price int64
	// What the fare allows.
	Refundable    bool
	ChangeFee     bool
	SaturdayNight bool // the fare wants one
	// Channel is where the offer is bought; it differs from the
	// traveller's own when the carrier is not sold there.
	Channel string
	// Partner says a carrier of the traveller's own programme's alliance
	// flies it, when the seller is not that carrier itself.
	Partner bool
	// Bonus is what the place a channel gives the offer on its display is
	// worth, in cents: the seller is paid to show it first.
	Bonus int64
}

// Cost is what an offer costs a traveller in all: the price and the
// price they put on everything about it they would rather not have. ok
// is false when the fare is more than they will pay.
func (t Traveller) Cost(o Offer) (cost int64, ok bool) {
	if o.Price > t.Most {
		return 0, false
	}
	cost = o.Price + int64(o.Connections)*t.ConnectionCost
	if !o.Refundable {
		cost += t.NonRefundableCost
	}
	if o.ChangeFee {
		cost += t.ChangeCost
	}
	if o.SaturdayNight && !t.SaturdayNight {
		// They can have the fare by staying over a weekend they did not
		// plan to.
		cost += t.SaturdayCost
	}
	off := o.Depart - t.Preferred
	if off < 0 {
		off = -off
	}
	cost += int64(off) * t.HourCost / 60
	if o.Channel != t.Channel {
		cost += t.SwitchCost
	}
	cost -= o.Bonus
	if t.Loyal != "" && len(o.Carriers) > 0 {
		switch {
		case o.Carriers[0] == t.Loyal:
			cost -= t.LoyaltyValue
		case o.Partner:
			cost -= t.LoyaltyValue / 2
		}
	}
	return cost, true
}

// Choose is the offer a traveller takes: the one that costs them least
// in all among those they will pay for, or -1 when there is none.
func (t Traveller) Choose(offers []Offer) int {
	best, bestCost := -1, int64(0)
	for i, o := range offers {
		c, ok := t.Cost(o)
		if !ok {
			continue
		}
		if best < 0 || c < bestCost {
			best, bestCost = i, c
		}
	}
	return best
}

// Sells reports whether a traveller will sit in a booking class's
// cabin: the front-cabin traveller in theirs and no other, and everyone
// else behind. The classes are the industry's usual ones: F, A and P
// first; J, C, D, I and Z business.
func (t Traveller) Sells(class string) bool {
	return CabinOf(class) == t.Cabin
}

// CabinOf is the cabin a booking class is in: "F", "C" or "" for economy.
func CabinOf(class string) string {
	switch class {
	case "F", "A", "P":
		return "F"
	case "J", "C", "D", "I", "Z":
		return "C"
	}
	return ""
}

// Join enrols a traveller in a carrier's programme. Business travellers
// mostly belong to one, and what it is worth to them is an estimate: 15
// per cent of the lowest fare, which is about the fare premium a
// dominant carrier keeps at its own hub.
func (t *Traveller) Join(rng *rand.Rand, carrier string, base int64) {
	share := 0.25
	if t.Kind == Business {
		share = 0.65
	}
	if carrier == "" || rng.Float64() >= share {
		return
	}
	t.Loyal = carrier
	t.LoyaltyValue = base * 15 / 100
}
