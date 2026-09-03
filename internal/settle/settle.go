// Package settle is the Billing and Settlement Plan: the body between the
// agents and the airlines that takes what the agents sold and hands each
// airline its Airline Accounting/Sales data file -- the HOT -- to
// reconcile against its own records. In the world, the distribution
// systems are the agents (each a reporting office), the plan runs once a
// day over what they ticketed, and each carrier checks the file it is
// handed against the documents it knows it issued. The file itself is
// jetway's pkg/bsp, to IATA's public DISH 23 handbook.
package settle

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/bsp"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
)

// Agent is one reporting office: a distribution system and its records.
type Agent struct {
	// Designator is the system's code (1G, 1S...); Name its name.
	Designator, Name string
	Store            store.Store
}

// Airline is one ticketing airline the plan settles for.
type Airline struct {
	Designator string
	// Accounting is the three-digit numeric code its documents carry.
	Accounting string
	// Store is the carrier's book of record, for reconciliation; nil skips it.
	Store store.Store
}

// Plan is the settlement plan's configuration.
type Plan struct {
	// BSP is the plan's city code; Country the agents' country; Currency
	// the currency type the plan settles in, e.g. USD2.
	BSP, Country, Currency string
	// CommissionRate is the standard commission in hundredths of a percent.
	CommissionRate int
	// Sequence numbers the files the plan has produced, per airline.
	Sequence map[string]int
}

// Statement is what one airline is handed for one day: the file, and the
// reconciliation of it against the airline's own records.
type Statement struct {
	Airline      string
	File         *bsp.File
	Transactions int
	Gross        int64
	Remittance   int64
	Commission   int64
	// Reconciliation: documents the plan reports that the carrier also
	// holds, documents the plan reports that the carrier does not hold,
	// and documents the carrier holds that no agent reported.
	Matched, Unreported, Unknown int
}

// Summary is the plan's day across airlines, for the instruments.
type Summary struct {
	Day          time.Time            `json:"day"`
	Airlines     int                  `json:"airlines"`
	Transactions int                  `json:"transactions"`
	Gross        int64                `json:"gross"`
	Remittance   int64                `json:"remittance"`
	Commission   int64                `json:"commission"`
	Matched      int                  `json:"matched"`
	Unreported   int                  `json:"unreported"`
	Unknown      int                  `json:"unknown"`
	Statements   map[string]Statement `json:"-"`
}

// Run settles one day: every ticketed document the agents hold dated on
// or before the day, grouped by ticketing airline, as one HOT per
// airline, each reconciled against the airline's own records when it has
// any. Documents are matched by number, which is what the airline's
// revenue accounting matches on.
func (p *Plan) Run(ctx context.Context, day time.Time, agents []Agent, airlines []Airline) (*Summary, error) {
	if p.Sequence == nil {
		p.Sequence = map[string]int{}
	}
	byAcct := map[string]Airline{}
	for _, a := range airlines {
		byAcct[a.Accounting] = a
	}
	// The plan's view: per airline, per agent, the transactions.
	type key struct{ airline, agent string }
	sold := map[key][]bsp.Transaction{}
	reported := map[string]map[string]bool{} // airline -> document numbers
	agentNames := map[string]Agent{}
	for _, ag := range agents {
		if ag.Store == nil {
			continue
		}
		agentNames[ag.Designator] = ag
		recs, err := ag.Store.ListPNRs(ctx, 1_000_000)
		if err != nil {
			return nil, fmt.Errorf("settle: list %s records: %w", ag.Designator, err)
		}
		for _, rec := range recs {
			for _, tk := range rec.Tickets {
				if tk.Type != "" && tk.Type != pnr.DocTicket {
					continue // EMDs settle by their own records; not yet
				}
				al, ok := byAcct[tk.Number.AirlineCode]
				if !ok || tk.IssuedAt.After(day.Add(24*time.Hour)) {
					continue
				}
				tx := transactionFor(rec, tk, ag, p)
				k := key{al.Designator, ag.Designator}
				sold[k] = append(sold[k], tx)
				if reported[al.Designator] == nil {
					reported[al.Designator] = map[string]bool{}
				}
				reported[al.Designator][tk.Number.AirlineCode+tk.Number.Serial] = true
			}
		}
	}

	sum := &Summary{Day: day, Statements: map[string]Statement{}}
	for _, al := range airlines {
		var offices []bsp.Office
		codes := make([]string, 0, len(agentNames))
		for d := range agentNames {
			codes = append(codes, d)
		}
		sort.Strings(codes)
		n := 0
		for _, d := range codes {
			txs := sold[key{al.Designator, d}]
			if len(txs) == 0 {
				continue
			}
			sort.Slice(txs, func(i, j int) bool { return txs[i].Document < txs[j].Document })
			offices = append(offices, bsp.Office{Agent: agentCode(d), RemittanceEnd: day, Currency: p.Currency, Transactions: txs})
			n += len(txs)
		}
		if n == 0 {
			continue
		}
		p.Sequence[al.Designator]++
		f := &bsp.File{
			BSP: p.BSP, Airline: al.Accounting, Country: p.Country, Processed: day.Add(27 * time.Hour), Sequence: p.Sequence[al.Designator],
			Cycle:   bsp.Cycle{ProcessingWeek: fmt.Sprintf("%02d%d", day.Month(), (day.Day()-1)/7+1), Number: 1, Ending: day, ReportingEnd: day, Final: true},
			Offices: offices,
		}
		st := Statement{Airline: al.Designator, File: f, Transactions: n}
		for oi := range f.Offices {
			for ti := range f.Offices[oi].Transactions {
				tot := f.Offices[oi].Transactions[ti].Compute()
				st.Gross += tot.Gross
				st.Remittance += tot.Remittance
				st.Commission += tot.Commission
			}
		}
		if al.Store != nil {
			held, err := al.Store.ListPNRs(ctx, 1_000_000)
			if err != nil {
				return nil, fmt.Errorf("settle: list %s records: %w", al.Designator, err)
			}
			known := map[string]bool{}
			for _, rec := range held {
				for _, tk := range rec.Tickets {
					if tk.Number.AirlineCode == al.Accounting && (tk.Type == "" || tk.Type == pnr.DocTicket) {
						known[tk.Number.AirlineCode+tk.Number.Serial] = true
					}
				}
			}
			for doc := range reported[al.Designator] {
				if known[doc] {
					st.Matched++
				} else {
					st.Unreported++
				}
			}
			for doc := range known {
				if !reported[al.Designator][doc] {
					st.Unknown++
				}
			}
		}
		sum.Statements[al.Designator] = st
		sum.Airlines++
		sum.Transactions += n
		sum.Gross += st.Gross
		sum.Remittance += st.Remittance
		sum.Commission += st.Commission
		sum.Matched += st.Matched
		sum.Unreported += st.Unreported
		sum.Unknown += st.Unknown
	}
	return sum, nil
}

// transactionFor is one ticket as the plan reports it: the sale, the
// passenger's share of the price, the coupons as segments, paid in cash
// by the agent's customer.
func transactionFor(rec *pnr.PNR, tk pnr.Ticket, ag Agent, p *Plan) bsp.Transaction {
	doc := tk.Number.AirlineCode + tk.Number.Serial
	cd, _ := bsp.CheckDigit(doc)
	tx := bsp.Transaction{
		Code: bsp.TransSale, Issued: tk.IssuedAt, Document: doc, CheckDigit: cd,
		Agent: agentCode(ag.Designator), ReportingSystem: strings.ToUpper(ag.Designator) + "SL",
		Locator: rec.RecordLocator + "/" + ag.Designator, Currency: p.Currency, CommissionRate: p.CommissionRate,
		TicketingMode: "/", ServicingSystem: "",
	}
	var pax *pnr.Passenger
	for i := range rec.Passengers {
		if rec.Passengers[i].Ref == tk.PaxRef {
			pax = &rec.Passengers[i]
		}
	}
	if pax != nil {
		tx.Passenger = strings.ToUpper(pax.Surname + "/" + pax.Given)
		if pax.Title != "" {
			tx.Passenger += " " + strings.ToUpper(pax.Title)
		}
		tx.PassengerType = "ADT"
		switch pax.Type {
		case pnr.PaxChild:
			tx.PassengerType = "CHD"
		case pnr.PaxInfant:
			tx.PassengerType = "INF"
		}
	}
	// The price: this passenger's base when the pricing names it, else an
	// even share; taxes shared evenly across the record's passengers.
	if rec.Pricing != nil && len(rec.Passengers) > 0 {
		n := int64(len(rec.Passengers))
		base := rec.Pricing.Base / n
		for _, pp := range rec.Pricing.Passengers {
			if pp.Ref == tk.PaxRef {
				base = 0
				for _, a := range pp.Segments {
					base += a
				}
			}
		}
		tx.Fare = base
		if rec.Pricing.Taxes > 0 {
			tx.Taxes = []bsp.Tax{{Code: "XT", Amount: rec.Pricing.Taxes / n}}
		}
		cur := rec.Pricing.Currency
		if cur == "" {
			cur = strings.TrimRight(p.Currency, "0123456789")
		}
		tx.FareText = fmt.Sprintf("%s%d.%02d", cur, tx.Fare/100, tx.Fare%100)
		tot := tx.Fare
		for _, t := range tx.Taxes {
			tot += t.Amount
		}
		tx.TotalText = fmt.Sprintf("%s%d.%02d", cur, tot/100, tot%100)
	}
	coupons := ""
	for _, c := range tk.Coupons {
		var seg *pnr.Segment
		for i := range rec.Segments {
			if rec.Segments[i].Ref == c.SegmentRef {
				seg = &rec.Segments[i]
			}
		}
		if seg == nil {
			continue
		}
		coupons += "F"
		s := bsp.Segment{Coupon: c.Number, Stopover: "O", Origin: seg.Board, Destination: seg.Off, Carrier: seg.Carrier,
			Flight: seg.FlightNum, Class: seg.Class, Cabin: cabinOf(seg.Class), Status: "OK", FareBasis: seg.FareBasis}
		if !seg.Depart.IsZero() {
			s.Departs = seg.Depart
			if len(seg.DepartTime) == 4 {
				if t, err := time.Parse("1504", seg.DepartTime); err == nil {
					s.Departs = seg.Depart.Add(time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute)
					s.DepartsHasTime = true
				}
			}
		}
		if len(tx.Segments) < 4 {
			tx.Segments = append(tx.Segments, s)
		}
	}
	tx.Coupons = coupons
	if len(tx.Segments) > 0 {
		tx.Origin, tx.Destination = tx.Segments[0].Origin, tx.Segments[len(tx.Segments)-1].Destination
	}
	var tot int64 = tx.Fare
	for _, t := range tx.Taxes {
		tot += t.Amount
	}
	tx.Payments = []bsp.Payment{{Type: bsp.PaymentCash, Amount: tot}}
	return tx
}

func cabinOf(class string) string {
	switch class {
	case "F", "A", "P":
		return "F"
	case "J", "C", "D", "I", "Z":
		return "C"
	}
	return "Y"
}

// agentCode is the eight-digit agent numeric code the plan assigns a
// distribution system: seven digits from its designator and a modulus-7
// check digit, as the handbook lays the code out.
func agentCode(designator string) string {
	h := 0
	for _, r := range strings.ToUpper(designator) {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	seven := fmt.Sprintf("9%06d", h%1000000)
	cd, _ := bsp.CheckDigit(seven)
	return seven + fmt.Sprint(cd)
}
