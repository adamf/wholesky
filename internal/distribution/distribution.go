// Package distribution is what it costs a carrier to sell a ticket, by
// the channel it was sold through, and what a carrier can do about it.
//
// A ticket sold on the carrier's own site costs it a dollar or two and
// the card fee. The same ticket sold by an agency through a distribution
// system costs a booking fee for every flight segment, of which the
// system passes about half to the agency; an online agency or a travel
// management company may also be paid a share of the fare. The figures
// are from the public record: the booking fees in the findings of US
// Airways v. Sabre and United States v. Sabre and in the General
// Accounting Office's report of 2003, the systems' own revenue per
// booking in their annual reports, and the card costs IATA published
// for 2025. Where the record gives a range the value used is inside it
// and is called an estimate.
//
// A carrier has levers. It can add a charge to tickets sold through a
// channel, as Lufthansa did in 2015 (16 euros, and direct sales rose
// from 30 to 45 per cent in three years). It can keep its lowest fares
// to its own channels, as American tried in 2023 (and lost about a
// tenth of its agency sales, and went back). It can pay a channel more
// to be shown first. Or it can stay out of a channel, as low-cost
// carriers did and mostly still do.
package distribution

import (
	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/market"
	"github.com/adamf/wholesky/internal/refdata"
)

// Policy is how a carrier sells.
type Policy struct {
	// Sells is the channels the carrier is sold in besides its own.
	Sells map[string]bool `json:"sells"`
	// Surcharge is what the carrier adds to a ticket sold in a channel,
	// in cents: its distribution cost charge.
	Surcharge map[string]int64 `json:"surcharge,omitempty"`
	// WebOnly keeps the carrier's lowest fares to its own channels.
	WebOnly bool `json:"web_only,omitempty"`
	// Override is the share of the fare the carrier pays a channel on top
	// of the booking fee, in hundredths of a percent, to be sold harder.
	Override map[string]int `json:"override,omitempty"`
	// FullContent is a deal with the distribution systems: a lower booking
	// fee for a promise of every fare and no surcharge. American counted
	// 133 million dollars a year saved from the deals of 2006.
	FullContent bool `json:"full_content,omitempty"`
	// Connect says the carrier offers agencies a direct connection: an
	// agency that has one books at the carrier's own system over NDC and
	// no distribution system is paid.
	Connect bool `json:"connect,omitempty"`
	// DirectFirst holds the last seats of each class for the carrier's
	// own channels: a request that comes through a distribution system is
	// worth the fare less what the channel costs, and is sold only while
	// the class under it is still open. A full-content deal forbids it,
	// so a carrier that turns it on gives the deal up.
	DirectFirst bool `json:"direct_first,omitempty"`
}

// Default is how a carrier of a kind sells: a network carrier everywhere
// under a full-content deal, with the override a managed-travel channel
// expects; a low-cost carrier on its own site and through the online
// agencies; an ultra-low-cost carrier on its own site alone.
func Default(model equipment.Model) Policy {
	switch model {
	case equipment.UltraLowCost:
		return Policy{Sells: map[string]bool{}}
	case equipment.LowCost:
		// A low-cost carrier has no full-content deal to keep: its lowest
		// seats go to its own site first, and agencies that connect to it
		// directly are welcome.
		return Policy{Sells: map[string]bool{market.OTA: true, market.TMC: true}, Connect: true, DirectFirst: true}
	}
	return Policy{Sells: map[string]bool{market.OTA: true, market.TMC: true, market.Agency: true}, FullContent: true, Connect: true,
		Override: map[string]int{market.TMC: 200, market.OTA: 100}}
}

// Offered reports whether a carrier with this policy is sold in a
// channel. Every carrier sells direct.
func (p Policy) Offered(channel string) bool {
	return channel == market.Direct || p.Sells[channel]
}

// Cost is what one ticket's sale cost the carrier, in cents.
type Cost struct {
	// System is the distribution system's booking fees.
	System int64 `json:"system"`
	// Commission is what the carrier paid the seller: a base commission
	// where one is still paid, and any override.
	Commission int64 `json:"commission"`
	// Payment is the cost of taking the money.
	Payment int64 `json:"payment"`
	// Own is the carrier's own cost of a direct sale, and the settlement
	// plan's charge on an agency one.
	Own int64 `json:"own"`
	// Incentive is the part of System that the distribution system passes
	// on to the agency that made the booking. It is the system's cost and
	// the agency's income, and is not added to Total.
	Incentive int64 `json:"incentive,omitempty"`
}

// Total is the whole cost.
func (c Cost) Total() int64 { return c.System + c.Commission + c.Payment + c.Own }

// SegmentFee is the booking fee for a flight segment, in cents. In North America a
// large carrier with a full-content deal paid 2.30 dollars (the weighted
// average of American, Delta and United at Sabre, June 2019) and a
// mid-sized one at least 3.41 (US Airways' contract of 2011). Elsewhere
// the systems average about 6 euros a booking, less for a sale in the
// carrier's home market: 4 dollars at home and 6 abroad are estimates
// from Amadeus's 5.90 euros a booking in 2023.
func SegmentFee(region string, fullContent, home bool) int64 {
	return segmentFee(region, fullContent, home)
}

func segmentFee(region string, fullContent, home bool) int64 {
	if region == refdata.NorthAmerica {
		if fullContent {
			return 230
		}
		return 340
	}
	fee := int64(600)
	if home {
		fee = 400
	}
	if fullContent {
		fee = fee * 85 / 100
	}
	return fee
}

// baseCommission is the commission agencies are still paid on the fare,
// in hundredths of a percent: none in North America since 2002, none in
// Europe or Australasia, and one to five per cent elsewhere. Three is an
// estimate.
func baseCommission(region string) int {
	switch region {
	case refdata.NorthAmerica, refdata.Europe, refdata.Oceania:
		return 0
	}
	return 300
}

// NDC is the channel of an agency's sale over a direct connection to
// the carrier.
const NDC = "ndc"

// Sale is one ticket sold.
type Sale struct {
	Channel  string
	Segments int
	// Kind is the kind of agency that sold it, when the channel is a
	// direct connection: an online agency or a travel management company.
	Kind string
	// Fare is what the carrier is paid: the base fare and its own fees.
	Fare int64
	// Paid is everything the traveller paid, taxes in; the card fee is a
	// share of it.
	Paid int64
	// Region is the part of the world the sale was made in, and Home
	// whether that is the carrier's own.
	Region string
	Home   bool
}

// CostOf is what a sale cost a carrier with this policy.
//
// A direct sale costs 1.50 dollars (an estimate; the General Accounting
// Office put the all-in cost of a ticket sold on a carrier's own site at
// 11.75 dollars in 2002 with advertising and the card fee in it). Every
// sale costs two per cent of what was paid to take the money: IATA's
// blended figure for 2025 was 1.97. An agency sale costs the booking fee
// on every segment, 25 cents for the settlement plan, the base
// commission where there is one, and the carrier's override.
func (p Policy) CostOf(s Sale) Cost {
	c := Cost{Payment: s.Paid * 2 / 100}
	if s.Channel == market.Direct {
		c.Own = 150
		return c
	}
	fee := segmentFee(s.Region, p.FullContent, s.Home) * int64(s.Segments)
	if s.Channel == NDC {
		// An agency connected to the carrier directly. The carrier pays
		// its technology vendor about a tenth of a distribution system's
		// fee (Farelogix's price in United States v. Sabre; American put
		// its saving at 80 to 90 per cent) and pays the agency two dollars
		// a ticket for booking that way, which is what American paid from
		// 2017.
		c.System = fee * 15 / 100
		c.Own = 25
		c.Commission = 200 + s.Fare*int64(p.Override[s.Kind])/10000
		return c
	}
	c.System = fee
	c.Incentive = fee * IncentiveShare / 100
	c.Own = 25
	rate := p.Override[s.Channel]
	if s.Channel == market.Agency {
		rate += baseCommission(s.Region)
	}
	c.Commission = s.Fare * int64(rate) / 10000
	return c
}

// IncentiveShare is the per cent of a booking fee that a distribution
// system passes on to the agency. Sabre's cost of revenue, which is
// mostly incentives, was 2.93 to 3.11 dollars a booking in 2022 to 2024,
// and Amadeus's was 51 per cent of its air distribution revenue in 2023.
const IncentiveShare = 50

// Favour is what a channel's display is worth to a carrier that pays it
// an override, as a share of the fare: a carrier shown first is chosen
// more. American's own study in 1981 found the first line of an agent's
// screen took more than half the bookings and the first screen 92 per
// cent. One point of override is taken to be worth 1.2 per cent of the
// fare in the traveller's eyes, which moves about 3 per cent of a
// contested agency's bookings. An estimate inside the 2 to 5 per cent
// the record suggests.
func (p Policy) Favour(channel string, fare int64) int64 {
	return fare * int64(p.Override[channel]) * 12 / 1000 / 100
}
