package noc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TAF is one terminal aerodrome forecast as aviationweather.gov's data
// API returns it in JSON: the station, its validity, and the forecast's
// groups, each decoded.
type TAF struct {
	ICAO   string  `json:"icaoId"`
	Raw    string  `json:"rawTAF"`
	From   int64   `json:"validTimeFrom"`
	To     int64   `json:"validTimeTo"`
	Groups []Group `json:"fcsts"`
}

// Group is one forecast group: the base, a change (FM, BECMG, TEMPO) or a
// probability (PROB30, PROB40, alone or with TEMPO).
type Group struct {
	From        int64       `json:"timeFrom"`
	To          int64       `json:"timeTo"`
	Change      *string     `json:"fcstChange"`
	Probability *int        `json:"probability"`
	Visibility  any         `json:"visib"`
	Weather     *string     `json:"wxString"`
	Gust        *int        `json:"wgst"`
	Clouds      []CloudBase `json:"clouds"`
}

// CloudBase is a layer: cover and base in feet.
type CloudBase struct {
	Cover string `json:"cover"`
	Base  *int   `json:"base"`
}

// ParseTAFs reads the data API's JSON.
func ParseTAFs(r io.Reader) ([]TAF, error) {
	var out []TAF
	if err := json.NewDecoder(io.LimitReader(r, 4<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("taf: %w", err)
	}
	return out, nil
}

// FetchTAFs asks aviationweather.gov for the stations' current forecasts.
// The service is free and asks no key; it is the US National Weather
// Service's, and covers stations worldwide.
func FetchTAFs(ctx context.Context, client *http.Client, icao []string) ([]TAF, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	u := "https://aviationweather.gov/api/data/taf?format=json&ids=" + url.QueryEscape(strings.Join(icao, ","))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "wholesky (github.com/adamf/wholesky)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aviationweather.gov: %s", resp.Status)
	}
	return ParseTAFs(resp.Body)
}

// Category is a flight category by the National Weather Service's
// definitions: LIFR below a 500-foot ceiling or a mile of visibility; IFR
// below 1,000 feet or three miles; MVFR to 3,000 feet and five miles;
// VFR above.
func (g Group) Category() string {
	ceiling := math.MaxInt32
	for _, c := range g.Clouds {
		if (c.Cover == "BKN" || c.Cover == "OVC" || c.Cover == "OVX" || c.Cover == "VV") && c.Base != nil {
			ceiling = min(ceiling, *c.Base)
		}
	}
	vis, known := g.visibility()
	switch {
	case ceiling < 500 || known && vis < 1:
		return "LIFR"
	case ceiling < 1000 || known && vis < 3:
		return "IFR"
	case ceiling <= 3000 || known && vis <= 5:
		return "MVFR"
	}
	return "VFR"
}

func (g Group) visibility() (float64, bool) {
	switch v := g.Visibility.(type) {
	case float64:
		return v, true
	case string:
		if strings.HasSuffix(v, "+") {
			f, err := strconv.ParseFloat(strings.TrimSuffix(v, "+"), 64)
			return f + 0.5, err == nil
		}
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
	return 0, false
}

// Factor is the arrival rate a group's weather leaves an airport, as a
// share of its rate in good weather. Airports publish their own arrival
// rates for each runway configuration and weather; this coarse mapping by
// category and phenomenon is the world's: instrument conditions take a
// quarter to two fifths of the rate, thunderstorms half, snow and freezing
// rain more, gusts a fifth.
func (g Group) Factor() float64 {
	f := map[string]float64{"VFR": 1, "MVFR": 0.9, "IFR": 0.75, "LIFR": 0.6}[g.Category()]
	wx := ""
	if g.Weather != nil {
		wx = *g.Weather
	}
	switch {
	case strings.Contains(wx, "FZRA"), strings.Contains(wx, "FZDZ"):
		f = math.Min(f, 0.45)
	case strings.Contains(wx, "TS"):
		f = math.Min(f, 0.5)
	case strings.Contains(wx, "SN"):
		f = math.Min(f, 0.6)
	case strings.Contains(wx, "+RA"):
		f = math.Min(f, 0.85)
	}
	if g.Gust != nil {
		switch {
		case *g.Gust >= 35:
			f *= 0.8
		case *g.Gust >= 25:
			f *= 0.95
		}
	}
	return f
}

// Kind names the group's change: "base", "FM", "BECMG", "TEMPO", "PROB".
func (g Group) Kind() string {
	if g.Change == nil || *g.Change == "" {
		return "base"
	}
	return *g.Change
}

// FromTAFs turns forecasts into a scenario: each group that cuts an
// airport's rate below 95 per cent becomes a weather event over the
// group's period, with the forecast's own uncertainty. A PROB30 or PROB40
// group happens with that probability. A TEMPO group's conditions come
// and go: the National Weather Service's TAF training (lesson 4.4) says
// each occurrence lasts an hour or less and together they cover less
// than half the period, so the event lasts between a sixth and a half of
// the period, starting anywhere in it. A base or FM group holds
// for its period, its onset known to within half an hour. Times are
// minutes from the day's midnight UTC; iata maps the stations.
func FromTAFs(tafs []TAF, day time.Time, iata func(icao string) string) Scenario {
	sc := Scenario{Name: "today's forecast"}
	midnight := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).Unix()
	for _, t := range tafs {
		ap := iata(t.ICAO)
		if ap == "" {
			continue
		}
		for _, g := range t.Groups {
			f := g.Factor()
			if f >= 0.95 {
				continue
			}
			start := float64(g.From-midnight) / 60
			end := float64(g.To-midnight) / 60
			if end <= 0 || start >= 30*60 {
				continue
			}
			start = math.Max(0, start)
			period := end - start
			e := Event{Kind: KindWeather, Airport: ap, Name: fmt.Sprintf("%s %s %s", t.ICAO, g.Kind(), g.Category()),
				Factor: Tri(math.Max(0.05, f-0.1), f, math.Min(1, f+0.1))}
			if g.Probability != nil {
				e.Probability = float64(*g.Probability) / 100
			}
			if g.Kind() == "TEMPO" {
				e.Onset = Dist{Kind: "uniform", A: start, B: math.Max(start, end-period/6)}
				e.Duration = Tri(period/6, period/3, period/2)
			} else {
				e.Onset = Tri(start-30, start, start+30)
				e.Duration = Tri(period*0.8, period, period*1.1)
			}
			sc.Events = append(sc.Events, e)
		}
	}
	return sc
}
