package workforce

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/crew"
)

// Unit is a bargaining unit: the employees one agreement covers.
type Unit string

const (
	Pilots           Unit = "pilots"
	FlightAttendants Unit = "flight attendants"
	Ground           Unit = "ground"
)

// Status is where an agreement stands.
type Status string

const (
	// InForce is an agreement inside its term.
	InForce Status = "in force"
	// Amendable is an agreement past its amendable date that neither side
	// has opened. Under the US Railway Labor Act it stays in force until a
	// new one is made; the union may still ask.
	Amendable Status = "amendable"
	// Negotiating is an agreement under talks.
	Negotiating Status = "in negotiation"
	// Impasse is talks that have broken down: the members work to rule
	// and call in sick more often.
	Impasse Status = "impasse"
)

// Contract is one labour agreement at one carrier. The pay arithmetic is
// jetway's crew.PayRules; the scale is the rate by fleet and years of
// service. Every number is synthetic, drawn from the carrier's code and
// scaled by its country, and shaped like published North American
// agreements: a captain's top rate on a narrowbody near 330 dollars an
// hour at a US carrier, a first officer's at about two thirds of the
// captain's, a flight attendant's near 60.
type Contract struct {
	Carrier string `json:"carrier"`
	Unit    Unit   `json:"unit"`
	// Union is a description, not a name: the world invents no unions.
	Union     string    `json:"union"`
	Signed    time.Time `json:"signed"`
	Amendable time.Time `json:"amendable"`
	Status    Status    `json:"status"`
	// TopRate is the hourly rate at the top of the scale on each fleet,
	// for the senior position (captain, purser), in cents.
	TopRate map[Fleet]int64 `json:"top_rate"`
	// JuniorShare is the second position's rate as a share of the senior
	// one's: the first officer's, the flight attendant's.
	JuniorShare float64 `json:"junior_share"`
	// Pay is the agreement's credit rules; Rate is set per member.
	Pay crew.PayRules `json:"pay"`
	// ReserveShare is the share of the unit that holds reserve lines.
	ReserveShare float64 `json:"reserve_share"`
	// JuniorAllowed says the carrier may assign a member on a day off.
	JuniorAllowed bool `json:"junior_allowed"`
	// SickRate is the share of members who call in sick on a day while
	// the agreement is in force.
	SickRate float64 `json:"sick_rate"`
	// GrievanceCost is what a grievance costs when the carrier loses it,
	// with its share of arbitration.
	GrievanceCost int64 `json:"grievance_cost"`
	// Raise is what the agreement has added to the scale during the run,
	// as a factor.
	Raise float64 `json:"raise"`
	// Proposal is the union's open proposal, a raise, when there is one.
	Proposal float64 `json:"proposal,omitempty"`
}

// countryLevel scales a carrier's pay to its market. The world knows the
// carrier's country, not its labour market, so the levels are coarse: the
// United States and Canada, western Europe and the Gulf, everywhere else.
func countryLevel(country string) float64 {
	switch country {
	case "United States", "Canada":
		return 1.0
	case "United Kingdom", "Ireland", "France", "Germany", "Netherlands", "Belgium", "Luxembourg", "Switzerland",
		"Austria", "Denmark", "Norway", "Sweden", "Finland", "Iceland", "Italy", "Spain", "Portugal",
		"United Arab Emirates", "Qatar", "Australia", "New Zealand", "Japan", "Singapore", "Hong Kong":
		return 0.8
	default:
		return 0.5
	}
}

// draw is a deterministic number in [0, 1) from the parts.
func draw(parts ...string) float64 {
	return float64(hash(parts...)>>11) / float64(1<<53)
}

func hash(parts ...string) uint64 {
	var h uint64 = 1469598103934665603
	for _, s := range parts {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= 1099511628211
		}
		h ^= '/'
		h *= 1099511628211
	}
	return h
}

// statusOf draws an agreement's standing: about two thirds in force, the
// rest amendable, in talks or at an impasse.
func statusOf(carrier string, u Unit) Status {
	switch x := draw(carrier, string(u), "status"); {
	case x < 0.66:
		return InForce
	case x < 0.80:
		return Amendable
	case x < 0.93:
		return Negotiating
	default:
		return Impasse
	}
}

// NewContract draws a carrier's agreement for a unit.
func NewContract(carrier, country string, u Unit, day time.Time) *Contract {
	lvl := countryLevel(country) * (0.85 + 0.3*draw(carrier, string(u), "level"))
	c := &Contract{Carrier: carrier, Unit: u, Status: statusOf(carrier, u), Raise: 1}
	term := 4 + int(draw(carrier, string(u), "term")*2)
	signedAgo := int(draw(carrier, string(u), "signed") * float64(term+2) * 365)
	c.Signed = day.AddDate(0, 0, -signedAgo)
	c.Amendable = c.Signed.AddDate(term, 0, 0)
	if c.Status == InForce && !c.Amendable.After(day) {
		c.Amendable = day.AddDate(0, 0, 30+int(draw(carrier, string(u), "left")*700))
	}
	if c.Status != InForce && c.Amendable.After(day) {
		c.Amendable = day.AddDate(0, 0, -30-int(draw(carrier, string(u), "past")*400))
	}
	cents := func(dollars float64) int64 { return int64(math.Round(dollars * lvl * 100)) }
	switch u {
	case Pilots:
		c.Union = "pilots' association"
		c.TopRate = map[Fleet]int64{Regional: cents(190), Narrowbody: cents(330), Widebody: cents(380)}
		c.JuniorShare = 0.66
		c.Pay = crew.PayRules{MinDay: 5*time.Hour + 15*time.Minute, DutyRig: 0.5, TripRig: 1 / 3.5, PayProtected: true,
			ExtensionPremium: 1.5, JuniorPremium: 2, ReserveCredit: 4*time.Hour + 30*time.Minute, PerDiem: cents(2.7), DeadheadShare: 1}
		c.ReserveShare = 0.12
		c.SickRate = 0.025
		c.GrievanceCost = cents(6000)
	case FlightAttendants:
		c.Union = "flight attendants' union"
		c.TopRate = map[Fleet]int64{Regional: cents(48), Narrowbody: cents(62), Widebody: cents(66)}
		c.JuniorShare = 0.9
		c.Pay = crew.PayRules{MinDay: 5 * time.Hour, DutyRig: 0.5, TripRig: 1 / 3.5, PayProtected: true,
			ExtensionPremium: 1.5, JuniorPremium: 2, ReserveCredit: 4 * time.Hour, PerDiem: cents(2.5), DeadheadShare: 1}
		c.ReserveShare = 0.15
		c.SickRate = 0.04
		c.GrievanceCost = cents(2500)
	case Ground:
		c.Union = "ground staff union"
		c.TopRate = map[Fleet]int64{Regional: cents(26), Narrowbody: cents(28), Widebody: cents(30)}
		c.JuniorShare = 1
		c.Pay = crew.PayRules{ExtensionPremium: 1.5}
		c.SickRate = 0.035
		c.GrievanceCost = cents(1500)
	}
	c.JuniorAllowed = c.Status != Impasse
	if c.Status != InForce {
		c.Proposal = 0.04 + 0.1*draw(carrier, string(u), "ask")
	}
	return c
}

// Rate is a member's hourly rate in cents: the scale's top rate for the
// fleet, the junior position's share of it, and a step for each year of
// service up to twelve, where the scale tops out.
func (c *Contract) Rate(f Fleet, senior bool, years int) int64 {
	top := float64(c.TopRate[f]) * c.Raise
	if !senior {
		top *= c.JuniorShare
	}
	step := 0.55 + 0.45*math.Min(float64(years), 12)/12
	return int64(math.Round(top * step))
}

// Rules is the agreement's pay rules at a member's rate.
func (c *Contract) Rules(f Fleet, senior bool, years int) crew.PayRules {
	r := c.Pay
	r.Rate = c.Rate(f, senior, years)
	return r
}

// SickToday is the share calling in sick under the agreement's standing:
// talks raise it, an impasse more than doubles it.
func (c *Contract) SickToday() float64 {
	switch c.Status {
	case Negotiating:
		return c.SickRate * 1.4
	case Amendable:
		return c.SickRate * 1.15
	case Impasse:
		return c.SickRate * 2.6
	}
	return c.SickRate
}

// GrievanceOdds is the chance the union grieves a day-off assignment.
func (c *Contract) GrievanceOdds() float64 {
	switch c.Status {
	case Impasse:
		return 0.9
	case Negotiating:
		return 0.6
	case Amendable:
		return 0.45
	}
	return 0.3
}

// Settle closes the open proposal at the given raise: the scale moves and
// the agreement is in force again for four years.
func (c *Contract) Settle(raise float64, day time.Time) {
	c.Raise *= 1 + raise
	c.Status = InForce
	c.Proposal = 0
	c.Signed = day
	c.Amendable = day.AddDate(4, 0, 0)
	c.JuniorAllowed = true
}

// Describe is one line about the agreement.
func (c *Contract) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s, amendable %s", c.Unit, c.Status, c.Amendable.Format("2 Jan 2006"))
	if c.Proposal > 0 {
		fmt.Fprintf(&b, "; the %s asks %.1f%%", c.Union, c.Proposal*100)
	}
	return b.String()
}
