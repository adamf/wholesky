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
	// Refunds is how many of the transactions are refunds.
	Refunds    int
	Gross      int64
	Remittance int64
	Commission int64
	// Reconciliation: documents the plan reports that the carrier also
	// holds, documents the plan reports that the carrier does not hold,
	// documents the carrier holds that no agent reported, and documents
	// reported from the carrier's copy alone because the agent's book is
	// not on this machine (Unverified).
	Matched, Unreported, Unknown, Unverified int
	// Peer is the machine that holds this statement's file when it is not
	// this one, for a federated view.
	Peer string
}

// Summary is the plan's day across airlines, for the instruments.
type Summary struct {
	Day          time.Time            `json:"day"`
	Airlines     int                  `json:"airlines"`
	Transactions int                  `json:"transactions"`
	Refunds      int                  `json:"refunds"`
	Gross        int64                `json:"gross"`
	Remittance   int64                `json:"remittance"`
	Commission   int64                `json:"commission"`
	Matched      int                  `json:"matched"`
	Unreported   int                  `json:"unreported"`
	Unknown      int                  `json:"unknown"`
	Unverified   int                  `json:"unverified"`
	Statements   map[string]Statement `json:"-"`
}

// Row is one airline's line of a settlement view, as the instruments and
// a federating core read it.
type Row struct {
	Airline      string `json:"airline"`
	Transactions int    `json:"transactions"`
	Refunds      int    `json:"refunds"`
	Gross        int64  `json:"gross"`
	Remittance   int64  `json:"remittance"`
	Commission   int64  `json:"commission"`
	Matched      int    `json:"matched"`
	Unreported   int    `json:"unreported"`
	Unknown      int    `json:"unknown"`
	Unverified   int    `json:"unverified"`
	File         string `json:"file"`
}

// View is the summary with its rows, the JSON a settlement endpoint serves.
type View struct {
	*Summary
	Rows []Row `json:"airlines_detail"`
}

// AsView lays the summary out for serving.
func (s *Summary) AsView() View {
	v := View{Summary: s}
	for code, st := range s.Statements {
		v.Rows = append(v.Rows, Row{Airline: code, Transactions: st.Transactions, Refunds: st.Refunds, Gross: st.Gross, Remittance: st.Remittance,
			Commission: st.Commission, Matched: st.Matched, Unreported: st.Unreported, Unknown: st.Unknown, Unverified: st.Unverified,
			File: "/settlement/" + code + ".hot"})
	}
	sort.Slice(v.Rows, func(i, j int) bool { return v.Rows[i].Gross > v.Rows[j].Gross })
	return v
}

// Merge folds several machines' views into one: the sums add, and each
// airline's row remembers the machine that holds its file.
func Merge(day time.Time, views map[string]View) *Summary {
	out := &Summary{Day: day, Statements: map[string]Statement{}}
	for peer, v := range views {
		if v.Summary == nil {
			continue
		}
		for _, r := range v.Rows {
			st := out.Statements[r.Airline]
			st.Airline = r.Airline
			st.Transactions += r.Transactions
			st.Refunds += r.Refunds
			st.Gross += r.Gross
			st.Remittance += r.Remittance
			st.Commission += r.Commission
			st.Matched += r.Matched
			st.Unreported += r.Unreported
			st.Unknown += r.Unknown
			st.Unverified += r.Unverified
			if st.Peer == "" || r.Transactions > 0 {
				st.Peer = peer
			}
			out.Statements[r.Airline] = st
		}
	}
	for _, st := range out.Statements {
		out.Airlines++
		out.Transactions += st.Transactions
		out.Refunds += st.Refunds
		out.Gross += st.Gross
		out.Remittance += st.Remittance
		out.Commission += st.Commission
		out.Matched += st.Matched
		out.Unreported += st.Unreported
		out.Unknown += st.Unknown
		out.Unverified += st.Unverified
	}
	return out
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
				if r, ok := refundFor(tx, tk, day); ok {
					sold[k] = append(sold[k], r)
				}
				if reported[al.Designator] == nil {
					reported[al.Designator] = map[string]bool{}
				}
				reported[al.Designator][tk.Number.AirlineCode+tk.Number.Serial] = true
			}
		}
	}

	// The carriers' books hold the same sales from the other side: a
	// record sold through an agent carries the agent's locator or origin.
	// Where the agent's own book is not on this machine, the carrier's
	// copy is the report -- marked unverified, because nobody here has
	// seen both sides.
	agentSet := map[string]bool{}
	for _, ag := range agents {
		agentSet[strings.ToUpper(ag.Designator)] = true
		if _, ok := agentNames[ag.Designator]; !ok {
			agentNames[ag.Designator] = ag
		}
	}
	unverified := map[string]int{}
	for _, al := range airlines {
		if al.Store == nil {
			continue
		}
		held, err := al.Store.ListPNRs(ctx, 1_000_000)
		if err != nil {
			return nil, fmt.Errorf("settle: list %s records: %w", al.Designator, err)
		}
		for _, rec := range held {
			agent := soldThrough(rec, agentSet)
			if agent == "" {
				continue
			}
			ag := agentNames[agent]
			if ag.Store != nil {
				continue // the agent's own book is here and was read above
			}
			for _, tk := range rec.Tickets {
				if tk.Type != "" && tk.Type != pnr.DocTicket || tk.Number.AirlineCode != al.Accounting || tk.IssuedAt.After(day.Add(24*time.Hour)) {
					continue
				}
				doc := tk.Number.AirlineCode + tk.Number.Serial
				if reported[al.Designator][doc] {
					continue
				}
				if reported[al.Designator] == nil {
					reported[al.Designator] = map[string]bool{}
				}
				reported[al.Designator][doc] = true
				unverified[al.Designator]++
				k := key{al.Designator, agent}
				tx := transactionFor(rec, tk, ag, p)
				sold[k] = append(sold[k], tx)
				if r, ok := refundFor(tx, tk, day); ok {
					sold[k] = append(sold[k], r)
				}
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
		st := Statement{Airline: al.Designator, File: f, Transactions: n, Unverified: unverified[al.Designator]}
		for oi := range f.Offices {
			for ti := range f.Offices[oi].Transactions {
				if f.Offices[oi].Transactions[ti].Code == bsp.TransRefund {
					st.Refunds++
				}
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
			// What the carrier's own copy reported is not a match with
			// anything: it is one side only.
			st.Matched -= st.Unverified
			for doc := range known {
				if !reported[al.Designator][doc] {
					st.Unknown++
				}
			}
		}
		sum.Statements[al.Designator] = st
		sum.Airlines++
		sum.Transactions += n
		sum.Refunds += st.Refunds
		sum.Gross += st.Gross
		sum.Remittance += st.Remittance
		sum.Commission += st.Commission
		sum.Matched += st.Matched
		sum.Unreported += st.Unreported
		sum.Unknown += st.Unknown
		sum.Unverified += st.Unverified
	}
	return sum, nil
}

// WriteRET is the agent's side of the exchange: the Agent Reporting Data
// file one distribution system would send the plan for the day, every
// ticketed document in its book as a sale (and, when refunded, a refund),
// in the handbook's chapter 5 layout. It is what a real plan builds the
// HOT from; here the plan reads the books directly and the RET is served
// for the record.
func (p *Plan) WriteRET(ctx context.Context, day time.Time, ag Agent, airlines []Airline) (*bsp.RET, error) {
	if ag.Store == nil {
		return nil, fmt.Errorf("settle: agent %s has no book here", ag.Designator)
	}
	byAcct := map[string]Airline{}
	for _, a := range airlines {
		byAcct[a.Accounting] = a
	}
	recs, err := ag.Store.ListPNRs(ctx, 1_000_000)
	if err != nil {
		return nil, fmt.Errorf("settle: list %s records: %w", ag.Designator, err)
	}
	ret := &bsp.RET{PeriodEnd: day, System: strings.ToUpper(ag.Designator) + "SL", Country: p.Country, Processed: day.Add(23 * time.Hour), Sequence: 1}
	for _, rec := range recs {
		for _, tk := range rec.Tickets {
			if tk.Type != "" && tk.Type != pnr.DocTicket {
				continue
			}
			if _, ok := byAcct[tk.Number.AirlineCode]; !ok || tk.IssuedAt.After(day.Add(24*time.Hour)) {
				continue
			}
			tx := transactionFor(rec, tk, ag, p)
			tx.Compute()
			ret.Transactions = append(ret.Transactions, tx)
			if r, ok := refundFor(tx, tk, day); ok {
				r.Compute()
				ret.Transactions = append(ret.Transactions, r)
			}
		}
	}
	sort.Slice(ret.Transactions, func(i, j int) bool {
		if ret.Transactions[i].Document != ret.Transactions[j].Document {
			return ret.Transactions[i].Document < ret.Transactions[j].Document
		}
		return ret.Transactions[i].Code < ret.Transactions[j].Code
	})
	return ret, nil
}

// soldThrough names the agent a record was sold through, when it was: the
// system that originated it, or one whose locator it carries.
func soldThrough(rec *pnr.PNR, agents map[string]bool) string {
	if p := strings.ToUpper(rec.Origin.Party); agents[p] {
		return p
	}
	for _, l := range rec.Locators {
		if o := strings.ToUpper(l.Owner); agents[o] {
			return o
		}
	}
	return ""
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
	if tk.ExchangedFrom != nil {
		// A reissue: the document it replaces is the qualifying issue, and
		// its value is the form of payment. An even exchange collects
		// nothing new and earns no new commission.
		tx.OriginalDocument = tk.ExchangedFrom.AirlineCode + tk.ExchangedFrom.Serial
		tx.OriginalAgent = agentCode(ag.Designator)
		for _, o := range rec.Tickets {
			if o.Number == *tk.ExchangedFrom {
				tx.OriginalIssued = o.IssuedAt
			}
		}
		tx.CommissionRate = 0
		tx.Payments = []bsp.Payment{{Type: bsp.PaymentExchange, Amount: tot}}
		return tx
	}
	tx.Payments = []bsp.Payment{{Type: bsp.PaymentCash, Amount: tot}}
	return tx
}

// refundFor is the refund transaction a refunded document adds after its
// sale: the same document with every amount reversed, dated when it was
// refunded, so the commission the agent kept comes back with it. The
// handbook has refunds reported with negative amounts and the commission
// recalled positive, which the reversed signs give.
func refundFor(sale bsp.Transaction, tk pnr.Ticket, day time.Time) (bsp.Transaction, bool) {
	if !tk.Refunded() || tk.RefundedAt == nil || tk.RefundedAt.After(day.Add(24*time.Hour)) {
		return bsp.Transaction{}, false
	}
	r := sale
	r.Code = bsp.TransRefund
	r.Issued = *tk.RefundedAt
	r.Fare, r.Total, r.CommissionAmount = -sale.Fare, -sale.Total, 0
	r.Taxes = nil
	for _, t := range sale.Taxes {
		r.Taxes = append(r.Taxes, bsp.Tax{Code: t.Code, Amount: -t.Amount})
	}
	r.Payments = nil
	for _, pm := range sale.Payments {
		r.Payments = append(r.Payments, bsp.Payment{Type: pm.Type, Amount: -pm.Amount})
	}
	r.Segments = nil
	return r, true
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
