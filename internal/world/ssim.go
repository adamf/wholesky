package world

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/ssim"
)

// The world's schedule as the industry files it: an SSIM chapter 7 file
// per carrier, written back to back, and read back as the flights a day
// runs. A compiled world carries its own timetable in the manifest; the
// SSIM file is the same timetable in the form a carrier hands its
// distribution systems and airports, so a schedule can come in from
// outside the compiler -- a carrier's real file -- and fly.

// WriteSSIM writes every carrier's schedule for the day as chapter 7
// records. Operated legs carry a DEI 050 segment record naming the
// marketing flight when the leg is a codeshare; the marketing carrier's
// file carries the same leg with the operating airline disclosure L and a
// DEI 010 record naming the flight that operates it. Times are UTC, with
// each station's offset in the time variation fields from its zone.
func WriteSSIM(w io.Writer, m *Manifest, day time.Time) error {
	byIATA := m.ByIATA()
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	days := strings.Repeat(" ", 7)
	wd := int(day.Weekday())
	if wd == 0 {
		wd = 7
	}
	days = days[:wd-1] + strconv.Itoa(wd) + days[wd:]
	variation := func(iata string) string {
		a, ok := byIATA[iata]
		if !ok || a.TZ == "" {
			return "+0000"
		}
		loc, err := time.LoadLocation(a.TZ)
		if err != nil {
			return "+0000"
		}
		_, off := day.Add(12 * time.Hour).In(loc).Zone()
		sign := "+"
		if off < 0 {
			sign, off = "-", -off
		}
		return fmt.Sprintf("%s%02d%02d", sign, off/3600, off%3600/60)
	}
	names := map[string]string{}
	for _, c := range m.Carriers {
		names[c.Designator] = c.Name
	}
	files := map[string]*ssim.File{}
	file := func(code string) *ssim.File {
		f, ok := files[code]
		if !ok {
			created := day.AddDate(0, 0, -30)
			f = &ssim.File{Carrier: code, TimeMode: ssim.UTC, Season: season(day), From: day, To: day, Created: created, Released: created,
				Title: strings.ToUpper(strings.TrimSpace("WHOLESKY " + names[code])), Status: "C", Creator: "WHOLESKY"}
			files[code] = f
		}
		return f
	}
	for _, fl := range m.Flights {
		leg := ssim.FlightLeg{
			Carrier: fl.Carrier, Number: strings.TrimLeft(fl.Number, "0"), Variation: 1, Sequence: 1, ServiceType: "J",
			From: day, To: day, Days: days,
			Board: fl.From, STD: hhmm(fl.DepMin), DepVariation: variation(fl.From),
			Off: fl.To, STA: hhmm(fl.ArrMin), ArrVariation: variation(fl.To), ArrDateVariation: fl.ArrMin / 1440,
			Equipment: fl.Equipment, Configuration: fmt.Sprintf("Y%d", fl.Seats),
		}
		if leg.Number == "" {
			leg.Number = "0"
		}
		f := file(fl.Carrier)
		f.Legs = append(f.Legs, leg)
		if fl.Marketing != "" && fl.Marketing != fl.Carrier && fl.MarketingNumber != "" {
			mnum := strings.TrimLeft(fl.MarketingNumber, "0")
			f.Data = append(f.Data, ssim.SegmentData{Carrier: leg.Carrier, Number: leg.Number, Variation: 1, Sequence: 1, ServiceType: "J",
				DEI: ssim.DEIMarketingFlights, Board: fl.From, Off: fl.To, Data: fl.Marketing + " " + mnum})
			mf := file(fl.Marketing)
			ml := leg
			ml.Carrier, ml.Number, ml.Disclosure = fl.Marketing, mnum, "L"
			mf.Legs = append(mf.Legs, ml)
			mf.Data = append(mf.Data, ssim.SegmentData{Carrier: ml.Carrier, Number: ml.Number, Variation: 1, Sequence: 1, ServiceType: "J",
				DEI: ssim.DEIOperatingFlight, Board: fl.From, Off: fl.To, Data: leg.Carrier + " " + leg.Number})
		}
	}
	codes := make([]string, 0, len(files))
	for c := range files {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		if err := files[c].Write(w); err != nil {
			return err
		}
	}
	return nil
}

// LoadSSIM reads a schedule written as chapter 7 records -- one carrier's
// file or many back to back -- and returns the flights the day runs, with
// distances from the manifest's airports and the aircraft rotations
// chained the way the compiler chains them. Marketing legs (disclosure L)
// become the Marketing designator on the flight that operates them, not
// flights of their own; a local-time file is placed on UTC through its
// time variation fields.
func LoadSSIM(r io.Reader, m *Manifest) ([]Flight, error) {
	f, err := ssim.ParseFile(r)
	if err != nil {
		return nil, err
	}
	byIATA := m.ByIATA()
	var flights []Flight
	index := map[string]int{} // carrier+number+board -> flights index
	marketing := map[string][2]string{}
	for _, l := range f.Legs {
		if l.Disclosure == "L" {
			if op := f.OperatingFlight(l); op.Carrier != "" {
				marketing[op.Carrier+op.Number+"/"+l.Board] = [2]string{l.Carrier, pad4(l.Number)}
			}
			continue
		}
		dep, err := minutesUTC(l.STD, l.DepVariation, f.TimeMode)
		if err != nil {
			return nil, fmt.Errorf("ssim: %s%s departure: %w", l.Carrier, l.Number, err)
		}
		arr, err := minutesUTC(l.STA, l.ArrVariation, f.TimeMode)
		if err != nil {
			return nil, fmt.Errorf("ssim: %s%s arrival: %w", l.Carrier, l.Number, err)
		}
		arr += l.ArrDateVariation * 1440
		if arr < dep {
			arr += 1440
		}
		fl := Flight{Carrier: l.Carrier, Number: pad4(l.Number), From: l.Board, To: l.Off, DepMin: dep, ArrMin: arr, BlockMin: arr - dep,
			Equipment: l.Equipment, Seats: seatsOf(l.Configuration)}
		if a, b := byIATA[l.Board], byIATA[l.Off]; a != nil && b != nil {
			fl.KM = int(haversineKM(a.Lat, a.Lon, b.Lat, b.Lon) + 0.5)
		}
		index[l.Carrier+l.Number+"/"+l.Board] = len(flights)
		flights = append(flights, fl)
	}
	for _, d := range f.Data {
		if d.DEI != ssim.DEIMarketingFlights {
			continue
		}
		if i, ok := index[d.Carrier+d.Number+"/"+d.Board]; ok {
			mk := parseDesignator(d.Data)
			if mk[0] != "" {
				flights[i].Marketing, flights[i].MarketingNumber = mk[0], mk[1]
			}
		}
	}
	for k, mk := range marketing {
		if i, ok := index[k]; ok && flights[i].Marketing == "" {
			flights[i].Marketing, flights[i].MarketingNumber = mk[0], mk[1]
		}
	}
	assignTails(flights)
	return flights, nil
}

// SellingDate is the day a compiled world is sold and flown: the recorded
// day resolved to its next occurrence on a replay, thirty days out on a
// synthetic world, so bookings, sells and the flight day all name the
// same date.
func SellingDate(m *Manifest) time.Time {
	if m.Replay != nil && !m.Replay.Date.IsZero() {
		// The wire carries a day and a month, no year, and every system
		// resolves 26NOV to the next 26 November. The recorded day is flown
		// on that date, so the bookings the filler writes, the sells the
		// distribution systems make and the day the flight day flies all
		// name the same date.
		d, err := pnr.ResolveDate(strings.ToUpper(m.Replay.Date.Format("02Jan")), time.Now().UTC())
		if err == nil {
			return d
		}
		return m.Replay.Date
	}
	return time.Now().UTC().AddDate(0, 0, 30)
}

// season is the IATA season the day falls in: winter from the last Sunday
// of October to the last Saturday of March, named for the year it starts.
func season(day time.Time) string {
	y := day.Year()
	switch {
	case day.Month() >= time.November, day.Month() == time.October && day.Day() >= 25:
		return fmt.Sprintf("W%02d", y%100)
	case day.Month() <= time.March:
		return fmt.Sprintf("W%02d", (y-1)%100)
	}
	return fmt.Sprintf("S%02d", y%100)
}

func hhmm(min int) string {
	min = ((min % 1440) + 1440) % 1440
	return fmt.Sprintf("%02d%02d", min/60, min%60)
}

func pad4(n string) string {
	for len(n) < 4 {
		n = "0" + n
	}
	return n
}

// minutesUTC places an HHMM on the UTC day: as is in a UTC file, less the
// station's variation in a local-time one.
func minutesUTC(hm, variation string, mode ssim.TimeMode) (int, error) {
	if len(hm) != 4 {
		return 0, fmt.Errorf("time %q", hm)
	}
	h, err1 := strconv.Atoi(hm[:2])
	mn, err2 := strconv.Atoi(hm[2:])
	if err1 != nil || err2 != nil || h > 24 || mn > 59 {
		return 0, fmt.Errorf("time %q", hm)
	}
	min := h*60 + mn
	if mode == ssim.LocalTime && len(variation) == 5 {
		vh, _ := strconv.Atoi(variation[1:3])
		vm, _ := strconv.Atoi(variation[3:5])
		off := vh*60 + vm
		if variation[0] == '-' {
			off = -off
		}
		min -= off
	}
	return ((min % 1440) + 1440) % 1440, nil
}

// seatsOf sums the cabins of a configuration string such as C12Y150.
func seatsOf(conf string) int {
	total, num := 0, ""
	for _, r := range conf + " " {
		if r >= '0' && r <= '9' {
			num += string(r)
			continue
		}
		if num != "" {
			n, _ := strconv.Atoi(num)
			total += n
			num = ""
		}
	}
	return total
}

// parseDesignator splits "AS 3000" into the carrier and the zero-padded
// number.
func parseDesignator(s string) [2]string {
	fs := strings.Fields(strings.TrimSpace(s))
	switch len(fs) {
	case 2:
		return [2]string{fs[0], pad4(fs[1])}
	case 1:
		if len(fs[0]) > 2 {
			return [2]string{fs[0][:2], pad4(strings.TrimLeft(fs[0][2:], "0"))}
		}
	}
	return [2]string{}
}
