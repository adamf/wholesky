package equipment

import (
	"math"

	"github.com/adamf/wholesky/internal/fuel"
)

// A turnaround is three things done at once, and it takes as long as the
// slowest. Passengers leave by the doors, the cabin is made ready, and
// the next passengers board. The hold is emptied and filled. The tanks
// are filled for the next leg. The rates are the manufacturers', from
// the terminal servicing charts in their airport planning documents:
// a single-aisle aircraft empties at 18 to 20 passengers a minute
// through one door and fills at 12; a twin-aisle at 25 and 15 a door;
// a belt loader takes 15 bags a minute off and puts 10 on; an A320's
// hose delivers 1,250 litres a minute. What differs between carriers is
// how many doors they use, what they do to the cabin, and how much
// spare time they schedule on top.

// Task is one job of a turnaround, in minutes from the moment the
// aircraft is on its stand.
type Task struct {
	Name  string  `json:"name"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	// Critical marks the tasks on the path that sets the turnaround.
	Critical bool `json:"critical,omitempty"`
}

// Turn is one turnaround worked out.
type Turn struct {
	// Minimum is the least time the aircraft needs on the stand.
	Minimum float64 `json:"minimum"`
	// Scheduled is the minimum with the carrier's buffer.
	Scheduled float64 `json:"scheduled"`
	// Path names what set the minimum: passengers, hold or fuel.
	Path  string `json:"path"`
	Tasks []Task `json:"tasks"`
}

// Ground is what a turnaround is worked out from.
type Ground struct {
	Type  string // IATA type code
	Seats int
	// Off and On are the passengers leaving and boarding.
	Off, On int
	// NextKM is the next leg's distance, for the fuel uplift.
	NextKM int
	Model  Model
	// LongHaul says the next or the last leg is long: the cabin is cleaned
	// and dressed in full and the galleys exchanged.
	LongHaul bool
}

// TurnFor works a turnaround out.
func TurnFor(g Ground) Turn {
	t := Type(g.Type, g.Seats)
	wide := t.Widebody()
	lcc := g.Model == LowCost || g.Model == UltraLowCost

	// Passenger path.
	doors, off, on, last := 1.0, 19.0, 12.0, 2.0
	bridge, away := 2.0, 1.5
	switch {
	case wide:
		doors, off, on, last = 2, 25, 15, 4
	case t.Body == "R" || t.Body == "T":
		off, on = 24, 16
		bridge, away = 1, 1
	case lcc:
		// Stairs at both doors.
		doors, off, on = 2, 17, 10
		bridge, away = 1, 1
	}
	cabin := 12.0 // cleaners and a galley exchange
	switch {
	case g.LongHaul:
		cabin = 30 + float64(g.Seats)/40 // a full clean, the trolleys, the search
	case lcc:
		cabin = 5 // the crew tidy
	case t.Body == "R" || t.Body == "T":
		cabin = 6
	case wide:
		cabin = 20
	}
	deplane := float64(g.Off) / (doors * off)
	board := float64(g.On)/(doors*on) + last
	p0 := bridge
	p1 := p0 + deplane
	p2 := p1 + cabin
	p3 := p2 + board
	passengers := p3 + away

	// Hold path: 1.2 bags a passenger. In bulk on a narrow-body, at 15
	// bags a minute off and 10 on with one loader a door; in containers
	// on a wide-body, 1.2 minutes off and 1.4 on for each of the forward
	// hold's, which is the half of them that sets the time.
	var bagsOff, bagsOn float64
	if wide {
		perULD := 35.0
		bagsOff = math.Ceil(float64(g.Off)*1.2/perULD/2) * 1.2
		bagsOn = math.Ceil(float64(g.On)*1.2/perULD/2) * 1.4
	} else {
		loaders := 2.0
		if t.Body == "R" || t.Body == "T" {
			loaders = 1
		}
		bagsOff = float64(g.Off) * 1.2 / (15 * loaders)
		bagsOn = float64(g.On) * 1.2 / (10 * loaders)
	}
	h0 := 2.0
	h1 := h0 + bagsOff
	h2 := h1 + bagsOn
	hold := h2 + 1.5

	// Fuel path: the next leg's burn and a tenth more, at the hose's
	// rate, with three minutes to connect and three to disconnect. A
	// low-cost carrier on a short leg often carries the fuel in and takes
	// none.
	litres := fuel.Burn(t.IATA, g.Seats, g.NextKM) * 1.1 / 0.8
	flow := 1250.0
	switch {
	case wide && t.Seats >= 400:
		flow = 5400
	case wide:
		flow = 3800
	case t.Seats >= 185 && t.Body == "N" && t.IATA == "752":
		flow = 2000
	case t.Body == "R" || t.Body == "T":
		flow = 1100
	}
	fuelling := litres / flow
	f0 := 3.0
	if !wide && !g.LongHaul {
		// Passengers are off before a narrow-body is fuelled, as most
		// stations require.
		f0 = math.Max(3, p1)
	}
	if lcc && g.NextKM > 0 && g.NextKM < 700 {
		fuelling = 0
	}
	f1 := f0 + fuelling
	fuelPath := f1 + 3
	if fuelling == 0 {
		fuelPath = 0
	}

	out := Turn{Path: "passengers", Minimum: passengers}
	if hold > out.Minimum {
		out.Path, out.Minimum = "hold", hold
	}
	if fuelPath > out.Minimum {
		out.Path, out.Minimum = "fuel", fuelPath
	}
	crit := func(path string) bool { return out.Path == path }
	out.Tasks = []Task{
		{Name: "bridge or stairs on", Start: 0, End: p0, Critical: crit("passengers")},
		{Name: "passengers off", Start: p0, End: p1, Critical: crit("passengers")},
		{Name: "cabin", Start: p1, End: p2, Critical: crit("passengers")},
		{Name: "passengers on", Start: p2, End: p3, Critical: crit("passengers")},
		{Name: "doors closed", Start: p3, End: passengers, Critical: crit("passengers")},
		{Name: "bags off", Start: h0, End: h1, Critical: crit("hold")},
		{Name: "bags on", Start: h1, End: h2, Critical: crit("hold")},
	}
	if fuelling > 0 {
		out.Tasks = append(out.Tasks, Task{Name: "fuel", Start: f0, End: f1, Critical: crit("fuel")})
	}
	out.Minimum = math.Round(out.Minimum*10) / 10
	// The buffer: nothing to five minutes for a low-cost carrier, ten at a
	// network carrier's spoke, half an hour and more on long haul.
	buffer := 10.0
	switch {
	case g.LongHaul:
		buffer = 30
	case g.Model == UltraLowCost:
		buffer = 2
	case lcc:
		buffer = 5
	}
	out.Scheduled = math.Ceil((out.Minimum+buffer)/5) * 5
	return out
}

// MinimumTurn is the least time a type needs between landing and its
// next departure for a carrier's way of working, with full loads: what a
// scheduler holds a rotation to, and what a late inbound aircraft cannot
// be turned faster than. It is the task model at a full cabin, held to
// no less than the manufacturer's own minimum.
func MinimumTurn(code string, seats int, model Model, km int) int {
	t := Type(code, seats)
	if seats <= 0 {
		seats = t.Seats
	}
	long := km >= 4500
	full := int(float64(seats) * 0.9)
	turn := TurnFor(Ground{Type: code, Seats: seats, Off: full, On: full, NextKM: km, Model: model, LongHaul: long})
	m := math.Max(turn.Minimum, t.TurnMinimum)
	return int(math.Ceil(m))
}
