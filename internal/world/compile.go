package world

import (
	"encoding/csv"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// CompileOptions steer a compilation.
type CompileOptions struct {
	// DataDir holds the OpenFlights snapshot: airports.dat, airlines.dat,
	// routes.dat.
	DataDir string
	// Seed makes the world reproducible. The same seed, data and options
	// always compile the same manifest.
	Seed int64
	// Scale is sigma: what fraction of the sky to keep. Carriers are dropped
	// smallest-first until roughly Scale of the world's flights remain, which
	// keeps the majors intact -- thinning every carrier equally would produce
	// airlines flying one flight a week, which is not what a small world is.
	Scale float64
	// Countries, when set, keeps only carriers based in these countries and
	// routes between airports in them. It is how "one continent" is spelled.
	Countries []string
	// MaxCarriers caps the carrier count after filtering, largest first.
	// Zero means no cap.
	MaxCarriers int
}

// Compile builds a manifest from the vendored data.
func Compile(opts CompileOptions) (*Manifest, error) {
	if opts.Scale <= 0 || opts.Scale > 1 {
		opts.Scale = 1
	}
	airports, err := readAirports(filepath.Join(opts.DataDir, "airports.dat"))
	if err != nil {
		return nil, err
	}
	airlines, err := readAirlines(filepath.Join(opts.DataDir, "airlines.dat"))
	if err != nil {
		return nil, err
	}
	routes, err := readRoutes(filepath.Join(opts.DataDir, "routes.dat"))
	if err != nil {
		return nil, err
	}

	inCountry := map[string]bool{}
	for _, c := range opts.Countries {
		inCountry[strings.ToLower(c)] = true
	}
	keepCountry := func(c string) bool {
		return len(inCountry) == 0 || inCountry[strings.ToLower(c)]
	}

	// Routes drive everything: an airport nobody flies to and an airline
	// flying nothing do not belong in the world.
	type carrierAgg struct {
		name, country, icao string
		routes              []rawRoute
		touch               map[string]int
	}
	agg := map[string]*carrierAgg{}
	for _, r := range routes {
		al, ok := airlines[r.airlineID]
		if !ok || al.iata == "" || len(al.iata) != 2 {
			continue
		}
		src, ok1 := airports[r.src]
		dst, ok2 := airports[r.dst]
		if !ok1 || !ok2 {
			continue
		}
		if !keepCountry(src.Country) || !keepCountry(dst.Country) || !keepCountry(al.country) {
			continue
		}
		a := agg[al.iata]
		if a == nil {
			a = &carrierAgg{name: al.name, country: al.country, icao: al.icao, touch: map[string]int{}}
			agg[al.iata] = a
		}
		a.routes = append(a.routes, r)
		a.touch[r.src]++
	}

	// Designator collisions: OpenFlights reuses IATA codes across defunct and
	// live airlines. Two characters is all a Type B address has, so the
	// smaller twin is dropped -- and because aggregation above already merged
	// same-code carriers, the drop happens implicitly; what remains is one
	// carrier per code with the union of their routes, which is the least
	// wrong of the cheap options.

	var carriers []Carrier
	for code, a := range agg {
		if len(a.routes) == 0 {
			continue
		}
		hub, best := "", -1
		for apt, n := range a.touch {
			if n > best || (n == best && apt < hub) {
				hub, best = apt, n
			}
		}
		format := "typeb"
		// Roughly a third of the world speaks EDIFACT for reservations in
		// this simulation, assigned by stable hash so a world compiles the
		// same way every time.
		if hash32(code)%3 == 0 {
			format = "edifact"
		}
		// Among the teletype carriers, a share dials in over MATIP -- the
		// airline transport -- rather than the plain framed link. MATIP
		// carries Type B only, so EDIFACT carriers stay on the plain link.
		transport := "tcp"
		if format == "typeb" && hash32(code)%5 == 1 {
			transport = "matip"
		}
		carriers = append(carriers, Carrier{
			Designator: code, Name: a.name, Country: a.country,
			ICAO:       icaoDesignator(code, a.icao),
			Hub:        hub,
			TTYAddress: ttyAddress(hub, code),
			Format:     format,
			Transport:  transport,
			Routes:     len(a.routes),
		})
	}
	sortCarriers(carriers)

	// Sigma: keep the majors whole, drop the tail. Count route entries as the
	// proxy for flights, walk from the largest carrier down until the kept
	// share reaches Scale.
	total := 0
	for _, c := range carriers {
		total += c.Routes
	}
	kept, sum := carriers[:0], 0
	for _, c := range carriers {
		if float64(sum) >= opts.Scale*float64(total) {
			break
		}
		if opts.MaxCarriers > 0 && len(kept) >= opts.MaxCarriers {
			break
		}
		kept = append(kept, c)
		sum += c.Routes
	}
	carriers = kept
	keptCode := map[string]bool{}
	for _, c := range carriers {
		keptCode[c.Designator] = true
	}

	// Flights: one daily rotation per directed route, plus a second frequency
	// where both ends are the carrier's own top airports -- a crude bank
	// structure that makes hubs look like hubs.
	var flights []Flight
	usedAirport := map[string]bool{}
	numbers := map[string]int{}
	for _, c := range carriers {
		a := agg[c.Designator]
		// Deterministic route order: the map walk above must not leak into
		// the manifest.
		rs := append([]rawRoute(nil), a.routes...)
		sortRoutes(rs)
		for _, r := range rs {
			src, dst := airports[r.src], airports[r.dst]
			km := haversineKM(src.Lat, src.Lon, dst.Lat, dst.Lon)
			if km < 30 {
				continue // OpenFlights holds a few degenerate pairs
			}
			eq, seats := equipmentFor(km)
			block := blockMinutes(km)

			// Frequency by how busy both ends are, capped by stage length:
			// trunk routes between busy stations run several rotations a day,
			// and nobody flies four daily long-hauls with one airframe. The
			// tiers are calibrated so the full world flies about what the
			// real one does in a day -- roughly a hundred thousand legs.
			touchMin := a.touch[r.src]
			if a.touch[r.dst] < touchMin {
				touchMin = a.touch[r.dst]
			}
			freq := 1
			switch {
			case touchMin >= 10:
				freq = 4
			case touchMin >= 3:
				freq = 3
			case touchMin >= 2:
				freq = 2
			}
			switch {
			case km > 3500:
				freq = 1
			case km > 1800 && freq > 2:
				freq = 2
			}
			for i := 0; i < freq; i++ {
				numbers[c.Designator]++
				n := numbers[c.Designator]
				if n > 9999 {
					break
				}
				// Departure spread deterministically across the operating day,
				// offset per frequency so the second rotation is an evening one.
				h := hash32(c.Designator + r.src + r.dst + strconv.Itoa(i) + strconv.FormatInt(opts.Seed, 10))
				dep := 6*60 + int(h%(16*60))
				flights = append(flights, Flight{
					Carrier: c.Designator, Number: fmt.Sprintf("%04d", n),
					From: r.src, To: r.dst,
					DepMin: dep, ArrMin: dep + block, BlockMin: block,
					Equipment: eq, Seats: seats, KM: int(km),
				})
				usedAirport[r.src], usedAirport[r.dst] = true, true
			}
		}
	}

	assignTails(flights)

	var apts []Airport
	for code := range usedAirport {
		apts = append(apts, *airports[code])
	}
	sortAirports(apts)

	m := &Manifest{
		Seed: opts.Seed, Scale: opts.Scale,
		Region:   strings.Join(opts.Countries, ","),
		Airports: apts, Carriers: carriers, Flights: flights,
	}
	return m, nil
}

// ttyAddress synthesises a reservations address: three letters of hub city
// code, the RM department, the carrier. Shaped like the real thing without
// claiming to be anyone's real address.
func ttyAddress(hub, code string) string {
	h := hub
	for len(h) < 3 {
		h += "X"
	}
	return h[:3] + "RM" + code
}

func hash32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// --- raw file readers ---------------------------------------------------

type rawAirline struct {
	iata, icao, name, country string
}

type rawRoute struct {
	airlineID string
	src, dst  string
}

func openCSV(path string) (*csv.Reader, *os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	return r, f, nil
}

func readAirports(path string) (map[string]*Airport, error) {
	r, f, err := openCSV(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]*Airport{}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 12 {
			continue
		}
		iata := rec[4]
		if len(iata) != 3 || iata == "\\N" {
			continue
		}
		lat, err1 := strconv.ParseFloat(rec[6], 64)
		lon, err2 := strconv.ParseFloat(rec[7], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		icao := rec[5]
		if len(icao) != 4 || icao == "\\N" {
			icao = ""
		}
		out[iata] = &Airport{
			IATA: iata, ICAO: icao, Name: rec[1], City: rec[2], Country: rec[3],
			Lat: lat, Lon: lon, TZ: rec[11],
		}
	}
	return out, nil
}

func readAirlines(path string) (map[string]rawAirline, error) {
	r, f, err := openCSV(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]rawAirline{}
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 8 || rec[7] != "Y" {
			continue // the active flag is unreliable, but inactive is a clear no
		}
		iata := rec[3]
		if len(iata) != 2 || iata == "\\N" || iata == "-" {
			continue
		}
		out[rec[0]] = rawAirline{iata: iata, icao: rec[4], name: rec[1], country: rec[6]}
	}
	return out, nil
}

func readRoutes(path string) ([]rawRoute, error) {
	r, f, err := openCSV(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []rawRoute
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 9 {
			continue
		}
		// Codeshare rows describe a marketing overlay on someone else's
		// metal; the operating flight appears as its own row. Multi-stop
		// entries are through-services this world does not model.
		if rec[6] == "Y" || rec[7] != "0" {
			continue
		}
		out = append(out, rawRoute{airlineID: rec[1], src: rec[2], dst: rec[4]})
	}
	return out, nil
}

func sortRoutes(rs []rawRoute) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && less(rs[j], rs[j-1]); j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

func less(a, b rawRoute) bool {
	if a.src != b.src {
		return a.src < b.src
	}
	return a.dst < b.dst
}

func sortAirports(as []Airport) {
	for i := 1; i < len(as); i++ {
		for j := i; j > 0 && as[j].IATA < as[j-1].IATA; j-- {
			as[j], as[j-1] = as[j-1], as[j]
		}
	}
}

// assignTails chains each carrier's flights into aircraft rotations. Greedy
// and deterministic: flights in departure order, each taken by a tail that
// last arrived at its origin and has had its turnaround, or by a fresh tail
// when none has. The result is what a real day of tails looks like from the
// movement stream: registrations that work their way around the network.
func assignTails(flights []Flight) {
	const turnaroundMin = 35

	byCarrier := map[string][]int{}
	for i, f := range flights {
		byCarrier[f.Carrier] = append(byCarrier[f.Carrier], i)
	}
	var codes []string
	for code := range byCarrier {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	for _, code := range codes {
		idx := byCarrier[code]
		sort.SliceStable(idx, func(a, b int) bool {
			fa, fb := flights[idx[a]], flights[idx[b]]
			if fa.DepMin != fb.DepMin {
				return fa.DepMin < fb.DepMin
			}
			return fa.Number < fb.Number
		})
		type tail struct {
			at      string
			freeMin int
		}
		var tails []tail
		for _, i := range idx {
			f := &flights[i]
			best := -1
			for j, tl := range tails {
				if tl.at != f.From || tl.freeMin+turnaroundMin > f.DepMin {
					continue
				}
				// The tightest connection wins, which is how schedules are
				// actually built: aircraft do not idle when they could fly.
				if best == -1 || tl.freeMin > tails[best].freeMin {
					best = j
				}
			}
			if best == -1 {
				tails = append(tails, tail{})
				best = len(tails) - 1
			}
			f.Tail = fmt.Sprintf("W%s%03d", code, best+1)
			tails[best] = tail{at: f.To, freeMin: f.ArrMin}
		}
	}
}
