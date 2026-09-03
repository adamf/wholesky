// Package interline bills between carriers: when a passenger flies one
// carrier on a ticket another carrier sold -- the recorded day's seven
// thousand codeshares -- the carrier that flew the coupon invoices the
// carrier whose document it was for the coupon's share of the fare, less
// an interline service charge for the seller's cost of sale. In the world
// the bill is raised from the books the machine holds, once a day and as
// the day goes, and each pair of carriers gets an invoice.
//
// The proration is jetway's pkg/prorate, straight rate by mileage; the
// invoice layout here is the world's own summary, not IATA's IS-XML, whose
// passenger record structures sit behind the SIS participation guide and
// were not consulted.
package interline

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/prorate"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/wholesky/internal/world"
)

// Book is one book of records the plan reads: a distribution system's or
// a carrier's.
type Book struct {
	Name  string
	Store store.Store
}

// Plan is the billing plan's configuration.
type Plan struct {
	// ServiceCharge is the interline service charge in hundredths of a
	// percent of the prorated amount; 900 is the nine per cent the
	// industry's default agreements carry.
	ServiceCharge int
	// Accounting names each carrier's three-digit code, which the
	// documents carry.
	Accounting func(designator string) string
}

// Line is one billed coupon.
type Line struct {
	Document string `json:"document"`
	Coupon   int    `json:"coupon"`
	// Flight is the operated flight; Sold the designator and number it
	// was sold as.
	Flight string `json:"flight"`
	Sold   string `json:"sold"`
	Sector string `json:"sector"`
	// Prorate is the coupon's share of the fare, ServiceCharge what the
	// biller keeps back for the seller, Net what the seller owes.
	Prorate       int64 `json:"prorate"`
	ServiceCharge int64 `json:"service_charge"`
	Net           int64 `json:"net"`
}

// Invoice is what one operating carrier bills one ticketing carrier.
type Invoice struct {
	Biller, Billed string
	Lines          []Line
	Coupons        int
	Prorate        int64
	ServiceCharge  int64
	Net            int64
	// Peer is the machine that holds this invoice when it is not this one.
	Peer string `json:"-"`
}

// Summary is the day's billing across carriers, for the instruments.
type Summary struct {
	Day           time.Time           `json:"day"`
	Invoices      int                 `json:"invoices"`
	Coupons       int                 `json:"coupons"`
	Prorate       int64               `json:"prorate"`
	ServiceCharge int64               `json:"service_charge"`
	Net           int64               `json:"net"`
	ByPair        map[string]*Invoice `json:"-"`
}

// Row is one invoice's line of a billing view.
type Row struct {
	Biller        string `json:"biller"`
	Billed        string `json:"billed"`
	Coupons       int    `json:"coupons"`
	Prorate       int64  `json:"prorate"`
	ServiceCharge int64  `json:"service_charge"`
	Net           int64  `json:"net"`
	File          string `json:"file"`
}

// View is the summary with its rows, the JSON a billing endpoint serves.
type View struct {
	*Summary
	Rows []Row `json:"invoices_detail"`
}

// PairKey names an invoice: biller then billed.
func PairKey(biller, billed string) string { return biller + "-" + billed }

// AsView lays the summary out for serving.
func (s *Summary) AsView() View {
	v := View{Summary: s}
	for k, inv := range s.ByPair {
		v.Rows = append(v.Rows, Row{Biller: inv.Biller, Billed: inv.Billed, Coupons: inv.Coupons, Prorate: inv.Prorate,
			ServiceCharge: inv.ServiceCharge, Net: inv.Net, File: "/billing/" + k + ".json"})
	}
	sort.Slice(v.Rows, func(i, j int) bool { return v.Rows[i].Net > v.Rows[j].Net })
	return v
}

// Merge folds several machines' views into one; each pair's row remembers
// the machine that holds its invoice.
func Merge(day time.Time, views map[string]View) *Summary {
	out := &Summary{Day: day, ByPair: map[string]*Invoice{}}
	for peer, v := range views {
		if v.Summary == nil {
			continue
		}
		for _, r := range v.Rows {
			k := PairKey(r.Biller, r.Billed)
			inv := out.ByPair[k]
			if inv == nil {
				inv = &Invoice{Biller: r.Biller, Billed: r.Billed, Peer: peer}
				out.ByPair[k] = inv
			}
			inv.Coupons += r.Coupons
			inv.Prorate += r.Prorate
			inv.ServiceCharge += r.ServiceCharge
			inv.Net += r.Net
		}
	}
	for _, inv := range out.ByPair {
		out.Invoices++
		out.Coupons += inv.Coupons
		out.Prorate += inv.Prorate
		out.ServiceCharge += inv.ServiceCharge
		out.Net += inv.Net
	}
	return out
}

// operated is a codeshare leg as the manifest knows it: who flew it and how
// far, keyed by the designator and number it was sold as and the board
// point.
type operated struct {
	carrier, number string
	km              int
}

// Run bills one day: every ticketed coupon in the books whose flight was
// sold under one carrier's code and flown by another, prorated by mileage
// across the ticket's coupons, less the service charge. A coupon is billed
// once however many books hold the record.
func (p *Plan) Run(ctx context.Context, day time.Time, m *world.Manifest, books []Book) (*Summary, error) {
	shares := map[string]operated{}
	kmOf := map[string]int{}
	for _, f := range m.Flights {
		kmOf[f.Carrier+strings.TrimLeft(f.Number, "0")+"/"+f.From] = f.KM
		if f.Marketing != "" && f.Marketing != f.Carrier && f.MarketingNumber != "" {
			shares[f.Marketing+strings.TrimLeft(f.MarketingNumber, "0")+"/"+f.From] = operated{carrier: f.Carrier, number: f.Number, km: f.KM}
		}
	}
	byAcct := map[string]string{}
	if p.Accounting != nil {
		for _, c := range m.Carriers {
			byAcct[p.Accounting(c.Designator)] = c.Designator
		}
	}
	sum := &Summary{Day: day, ByPair: map[string]*Invoice{}}
	billed := map[string]bool{} // document+coupon
	for _, b := range books {
		if b.Store == nil {
			continue
		}
		recs, err := b.Store.ListPNRs(ctx, 1_000_000)
		if err != nil {
			return nil, fmt.Errorf("interline: list %s records: %w", b.Name, err)
		}
		for _, rec := range recs {
			if rec.Pricing == nil || len(rec.Passengers) == 0 {
				continue
			}
			for _, tk := range rec.Tickets {
				if tk.Type != "" && tk.Type != pnr.DocTicket {
					continue
				}
				validating := byAcct[tk.Number.AirlineCode]
				if validating == "" {
					continue
				}
				// The ticket's coupons with their sectors' distances, for
				// the prorate; the fare is this passenger's share of the base.
				var coupons []prorate.Coupon
				var segs []*pnr.Segment
				for _, c := range tk.Coupons {
					seg := rec.SegmentByRef(c.SegmentRef)
					if seg == nil || seg.Type != pnr.SegmentAir {
						continue
					}
					key := seg.Carrier + strings.TrimLeft(seg.FlightNum, "0") + "/" + seg.Board
					km := kmOf[key]
					if op, ok := shares[key]; ok {
						km = op.km
					}
					coupons = append(coupons, prorate.Coupon{Number: c.Number, Carrier: seg.Carrier, Distance: km})
					segs = append(segs, seg)
				}
				if len(coupons) == 0 {
					continue
				}
				fare := rec.Pricing.Base / int64(len(rec.Passengers))
				for _, pp := range rec.Pricing.Passengers {
					if pp.Ref == tk.PaxRef {
						fare = 0
						for _, a := range pp.Segments {
							fare += a
						}
					}
				}
				prorated, err := prorate.Straight(fare, coupons)
				if err != nil {
					continue
				}
				doc := tk.Number.AirlineCode + tk.Number.Serial
				for i, seg := range segs {
					op, ok := shares[seg.Carrier+strings.TrimLeft(seg.FlightNum, "0")+"/"+seg.Board]
					if !ok || op.carrier == validating {
						continue // flown by the seller, or by the seller's own metal: nothing to bill
					}
					id := doc + "/" + fmt.Sprint(coupons[i].Number)
					if billed[id] {
						continue
					}
					billed[id] = true
					amount := prorated[i].Amount
					isc := prorate.ServiceCharge(amount, p.ServiceCharge)
					k := PairKey(op.carrier, validating)
					inv := sum.ByPair[k]
					if inv == nil {
						inv = &Invoice{Biller: op.carrier, Billed: validating}
						sum.ByPair[k] = inv
					}
					inv.Lines = append(inv.Lines, Line{Document: doc, Coupon: coupons[i].Number,
						Flight: op.carrier + op.number, Sold: seg.Carrier + seg.FlightNum, Sector: seg.Board + "-" + seg.Off,
						Prorate: amount, ServiceCharge: isc, Net: amount - isc})
					inv.Coupons++
					inv.Prorate += amount
					inv.ServiceCharge += isc
					inv.Net += amount - isc
				}
			}
		}
	}
	for _, inv := range sum.ByPair {
		sort.Slice(inv.Lines, func(i, j int) bool { return inv.Lines[i].Document < inv.Lines[j].Document })
		sum.Invoices++
		sum.Coupons += inv.Coupons
		sum.Prorate += inv.Prorate
		sum.ServiceCharge += inv.ServiceCharge
		sum.Net += inv.Net
	}
	return sum, nil
}
