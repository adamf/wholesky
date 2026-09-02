package world

import (
	"os"
	"path/filepath"
	"testing"
	_ "time/tzdata"
)

// A hand-written slice of the BTS file: a Southwest flight, a regional
// operating an Alaska codeshare, a cancelled flight, a diverted one, and a
// Honolulu evening departure that is the next UTC morning.
const btsFixture = `FlightDate,IATA_Code_Marketing_Airline,Flight_Number_Marketing_Airline,IATA_Code_Operating_Airline,Tail_Number,Flight_Number_Operating_Airline,Origin,Dest,CRSDepTime,DepTime,DepDelay,CRSArrTime,ArrTime,ArrDelay,Cancelled,CancellationCode,Diverted,CRSElapsedTime,ActualElapsedTime,Distance,CarrierDelay,WeatherDelay,NASDelay,SecurityDelay,LateAircraftDelay,Div1Airport
2025-11-26,WN,1234,WN,N8325D,1234,DAL,HOU,0700,0712,12.00,0800,0815,15.00,0.00,,0.00,60.00,63.00,239.00,0.00,0.00,3.00,0.00,12.00,
2025-11-26,AS,3000,OO,N405SY,3000,SJC,LAX,0851,0846,-5.00,1020,1013,-7.00,0.00,,0.00,89.00,87.00,308.00,,,,,,
2025-11-26,DL,401,DL,N123DL,401,ATL,LGA,1500,,,1720,,,1.00,B,0.00,140.00,,762.00,,,,,,
2025-11-26,UA,88,UA,N77UA,88,SFO,EWR,0900,0930,30.00,1730,1900,90.00,0.00,,1.00,330.00,,2565.00,,,,,,PIT
2025-11-26,HA,50,HA,N380HA,50,HNL,LAX,2200,2201,1.00,0530,0531,1.00,0.00,,0.00,330.00,330.00,2556.00,,,,,,
2025-11-25,WN,9,WN,N8999,9,DAL,HOU,0700,0700,0.00,0800,0800,0.00,0.00,,0.00,60.00,60.00,239.00,,,,,,
`

func TestCompileReplayReadsTheRecordedDay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "day.csv")
	if err := os.WriteFile(path, []byte(btsFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := CompileReplay(ReplayOptions{DataDir: "../../data", BTS: path, Date: "2025-11-26"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Replay == nil || m.Replay.Flights != 5 || m.Replay.Cancelled != 1 || m.Replay.Diverted != 1 || m.Replay.Tails != 5 {
		t.Fatalf("replay summary %+v", m.Replay)
	}
	if m.Replay.Date.Format("2006-01-02") != "2025-11-26" {
		t.Errorf("date %v", m.Replay.Date)
	}
	by := map[string]Flight{}
	for _, f := range m.Flights {
		by[f.Carrier+f.Number] = f
	}
	if len(by) != 5 {
		t.Fatalf("%d flights: the other day's row must be skipped", len(by))
	}

	// Dallas 07:00 CST is 13:00 UTC.
	wn := by["WN1234"]
	if wn.DepMin != 13*60 || wn.ArrMin != 14*60 || wn.BlockMin != 60 || wn.Tail != "N8325D" {
		t.Errorf("WN1234 %+v", wn)
	}
	if wn.Equipment != "73H" || wn.Seats != 174 {
		t.Errorf("Southwest flies 737s: %s %d", wn.Equipment, wn.Seats)
	}
	if wn.Actual == nil || wn.Actual.DepDelay != 12 || wn.Actual.ArrDelay != 15 || wn.Actual.LateAircraft != 12 || wn.Actual.NAS != 3 {
		t.Errorf("WN1234 actuals %+v", wn.Actual)
	}
	codes := wn.Actual.DelayCodes()
	if len(codes) != 2 || codes[0].Code != "93" || codes[0].Minutes != 12 || codes[1].Code != "81" {
		t.Errorf("delay codes %+v", codes)
	}

	// The regional flies the Alaska codeshare: operated by OO, sold as AS3000.
	oo := by["OO3000"]
	if oo.Marketing != "AS" || oo.MarketingNumber != "3000" || oo.Equipment != "E75" {
		t.Errorf("OO3000 %+v", oo)
	}
	if oo.Actual.DepDelay != -5 {
		t.Errorf("an early departure is a negative delay: %d", oo.Actual.DepDelay)
	}
	if wn.Marketing != "" {
		t.Errorf("a flight sold as itself has no codeshare: %q", wn.Marketing)
	}

	dl := by["DL0401"]
	if !dl.Actual.Cancelled || dl.Actual.CancelCode != "B" || dl.Actual.DepDelay != 0 {
		t.Errorf("DL401 %+v", dl.Actual)
	}
	ua := by["UA0088"]
	if !ua.Actual.Diverted || ua.Actual.DivertedTo != "PIT" || ua.Equipment != "321" {
		t.Errorf("UA88 %+v equipment %s", ua.Actual, ua.Equipment)
	}
	// Honolulu 22:00 HST is 08:00 UTC the next day: it wraps into the day.
	ha := by["HA0050"]
	if ha.DepMin != 8*60 || ha.ArrMin != 8*60+330 {
		t.Errorf("HA50 %d..%d", ha.DepMin, ha.ArrMin)
	}
	if ha.Equipment != "789" {
		t.Errorf("Hawaii is a wide body: %s", ha.Equipment)
	}

	// Carriers come from the record, named from the snapshot, hubbed where
	// they touch most; airports include the diversion field.
	if len(m.Carriers) != 5 {
		t.Errorf("%d carriers", len(m.Carriers))
	}
	seen := map[string]bool{}
	for _, a := range m.Airports {
		seen[a.IATA] = true
	}
	for _, want := range []string{"DAL", "HOU", "SJC", "LAX", "ATL", "LGA", "SFO", "EWR", "PIT", "HNL"} {
		if !seen[want] {
			t.Errorf("airport %s missing", want)
		}
	}
	for _, c := range m.Carriers {
		if c.Designator == "WN" && (c.Name == "WN" || c.Country != "United States") {
			t.Errorf("Southwest unnamed: %+v", c)
		}
	}
	// A synthetic flight has no record; RecordedDelay says so.
	if _, _, ok := (Flight{}).RecordedDelay(); ok {
		t.Error("a flight without actuals claims a recorded delay")
	}
}
