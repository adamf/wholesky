# Crews, agreements and the operations centre

Every carrier in the world has a roster of named crew members and the
labour agreements they work under. Every carrier also has a network
operations centre (NOC). The NOC watches the day's signals and projects
what a disruption will cost before it arrives. This document describes
the roster, the agreements, the operations centre and the projection
tools. It also says which parts follow a public text and which parts are
the world's own numbers.

## The roster

The compiler gives each carrier a schedule. `internal/workforce` sizes a
roster from it. The day plan cuts each tail's day into duties, and the
roster counts the captains, first officers and flight attendants that
those duties need, by crew base and by fleet. A carrier's crew bases are
its hub and up to 2 other stations, each with a tenth of its departures.
A duty that starts away from a base is flown by a crew from the hub on a
layover.

Each pool holds a fifth more members than the day needs, and the
agreement's share of reserves. Every line is 4 days on and 3 days off.
The members of a pool start the cycle on different days, spread evenly,
so each day of the week has the same share of the pool at work. Every
member has an employee number, a seniority, years of service and a
record of the last 28 days. The record holds each day's report, release
and block time. The block time of the 336 days before the record is
carried as one number. At boot the record is drawn from the member's
line, and some members have flown hard enough to be near a limit.

The whole sky has about 885,000 crew members. Their rosters take about
500 MB and 5 s to staff. `skyd -no-crew` turns the rosters off. The plan
then flies every duty with any crew and calls reserves at every hub, as
it did before rosters.

### Staffing a duty

The roster staffs each duty by name, in a fixed order of tails, so every
machine names the same crews. A duty needs 2 pilots. A long-haul duty of
8 hours or more needs 3, and one of more than 12 hours needs 4. The cabin
needs the flight attendants that 14 CFR 121.391(a) requires for the
seats: one for 10 to 50 seats, two for 51 to 100, and one more for each
50 seats or part of 50 above 100.

For each position the roster tries the line first, in turn, so that the
flying spreads. A member is a candidate only when jetway's `pkg/crew`
admits the member to the duty:

- Pilots are checked against 14 CFR Part 117: Tables A and B for the
  duty itself, 10 hours of rest before report (§117.25(e)), 30 hours free
  in the 168 hours before it (§117.25(b)), and the cumulative limits of
  §117.23. These limits are 100 hours of flight time in any 672
  consecutive hours, 1,000 hours in any 365 consecutive calendar days, 60
  flight duty period (FDP) hours in any 168 hours and 190 FDP hours in
  any 672 hours.
- Flight attendants are checked against 14 CFR 121.467(b): 14 hours of
  scheduled duty, or 16, 18 or 20 with more flight attendants than the
  minimum, 10 hours of rest after a duty of 14 hours or less and 12 after
  a longer one, and 24 hours free in any 7 calendar days.

When the line has nobody legal, a reserve on call covers the open time
before the day starts. When the reserves have nobody legal, the roster
assigns a line member on a day off, if the agreement allows it. That
member is paid the agreement's premium, and the union may file a
grievance. When nobody at all is legal, the plan cancels the duty's
legs for crew (code A).

The text of Part 117 and Part 121 is the eCFR's of 15 October 2026. The
jetway tests check each limit value by value. A mutation test checks
every rule.

### As the day runs

The plan checks every duty again with the day's delays. With a roster,
the check uses the named crew's own records. A duty can now go illegal in
three ways:

- The duty runs past Table B, even with the 2-hour extension.
- A delay pushes a pilot over a cumulative limit. §117.23 has no
  extension.
- A crew member calls in sick at report.

A pilot who cannot fly needs a reserve. The reserve must be on call at
the station, the station must be a crew base, and the reserve must be
legal for the rest of the duty. The flight then leaves 90 minutes later.
Away from a base, no reserve can reach the aircraft, and the flight is
cancelled for crew. A flight attendant who calls in sick at report does
not stop the flight while another flight attendant is aboard. The flight
goes one short with seats blocked.

Most sick calls come the evening before and keep the member off the
roster. One sick call in 12 comes at report. The share of members who
call in sick depends on the agreement's standing (see below).

Under 14 CFR 121.467(b)(14), a flight attendant is not over the limit
when flights scheduled to end within it run late for reasons beyond the
carrier's control. The roster therefore checks cabin crews on the
schedule, and delays do not time them out.

Part 117's Table C, the augmented crew's limits by rest facility, is not
transcribed in jetway. A long-haul duty is therefore checked against the
cumulative limits and the rest rules only.

### A new day

The world flies the same date again and again. At each day wrap the
roster files the day just flown into every member's record and moves the
lines on by one day. It draws the sick calls again. The plan then works
the day out again from the schedule and staffs it afresh. Hours flown
yesterday therefore count against today's limits, and a long day can
leave a pilot unable to fly an early duty the next morning. Seats'
cancellations, injected weather and standby crews belong to the day on
which they happened, and they do not carry over.

## The agreements

Each carrier has three agreements: pilots, flight attendants and ground
staff. Each agreement has a pay scale, credit rules, a reserve share, a
sick rate, a grievance cost, a signing date and an amendable date. The
numbers are synthetic. They are drawn from the carrier's code and scaled
by the carrier's country. The scale is shaped like a published North
American agreement. A captain on a narrowbody at the top of the scale
earns about 330 dollars an hour at a US carrier, a first officer about two
thirds of that, and a flight attendant about 60 dollars.

The credit rules are jetway's `crew.PayRules`. A duty is credited the
greatest of these four amounts:

- the block time, scheduled or flown, whichever is greater (pay
  protection),
- a duty rig of 1 hour for every 2 hours on duty,
- a trip rig of 1 hour for every 3.5 hours away from base,
- a minimum day.

That rule is the one that section 11.D of the American Airlines flight
attendants' On-Duty Contract Guide (APFA, April 2016) states. A jetway
test uses the guide's numbers. Credit beyond the schedule's is paid at
time and a half. A duty on a day off is paid double. Per diem is paid by
the hour away from base, and a crew that ends its day away from a base
sleeps in a hotel. A reserve on call who is not called is paid the
reserve guarantee.

### Standing

An agreement is in force, amendable, in negotiation or at an impasse.
About two thirds start in force. The standing changes the sick rate. Talks
raise it by two fifths, and an impasse raises it 2.6 times. The standing
also changes how often the union grieves a day-off assignment, from 3 in
10 in force to 9 in 10 at an impasse. At an impasse, the union allows no
day-off assignments.

An agreement that is not in force has an open proposal: a raise that the
union asks for. At 06:00 each day the labour relations department answers
it:

- Accept: the scale rises by the ask at once, and the agreement is in
  force for 4 years.
- Counter at half the ask: the union takes it about half the time. If the
  union does not take it, the talks go on and the union asks for less.
- Refuse: the talks break down. The union allows no day-off assignments,
  and sick calls more than double from the next day.

The autopilot counters. A seat that runs labour relations by hand gets a
decision.

## Costs on the scorecard

The scorecard prices the day's people from the roster:

- **crew**: the crews' credit at their rates, the per diem and the hotels.
  A cancelled duty is still paid, because of pay protection.
- **crew premiums**: extensions and day-off assignments.
- **reserves**: the guarantee of the reserves on call who were not called,
  and of the standby crews brought in at half again.
- **labour**: the grievances, at the agreement's cost.
- **ground staff**: overtime for the turn team kept on past a delay of 15
  minutes, and for the agents who rebook a cancelled flight's passengers.

The scorecard also prices what passengers are owed under the law that
covers their flight (`internal/rights`):

- Regulation (EC) 261/2004 and the United Kingdom's retained copy pay EUR
  250, 400 or 600 (GBP 220, 350 or 520) by distance. They pay for a
  cancellation announced on the day, and for an arrival 3 hours or more
  late. They do not pay when weather or air traffic control caused the
  disruption, which are extraordinary circumstances under Article 5(3).
  They halve the amount when the alternative arrives within 2, 3 or 4
  hours. They owe meals after 2, 3 or 4 hours whatever the cause, and a
  hotel when the wait runs overnight.
- United States law pays no compensation. 14 CFR 260 owes a refund when a
  passenger does not take the alternative after a cancellation or a delay
  of 3 hours domestic or 6 hours international. The large US carriers'
  published customer service commitments give meals and hotels when the
  carrier caused the disruption.

The amounts and thresholds are the regulations'. The text of 261/2004 is
the adopted text on legislation.gov.uk. The cost of a meal, a hotel, the
exchange rates and the share of passengers who take a refund are the
world's numbers.

## The operations centre

Each carrier's operations centre is at `/noc/<code>`. Its JSON is at
`/carrier/<code>/noc`. It shows these signals:

- the weather systems forecast or in force over the carrier's stations,
  with the carrier's arrivals in each,
- the Network Manager's regulations on the carrier's flights,
- the day so far: late, cancelled, cancelled for crew, slotted, reserves
  called, crews on the extension,
- the crews: who is flying, on call, called out or sick, the pilots with
  the least time left under a cumulative limit after today's duty, and the
  agreements,
- the events added to the day.

![United's operations centre at 10:59 on a six-carrier US world, with weather over Newark, Denver, Orlando and Chicago. A projection of 300 plays puts the expected cost of absorbing the weather at $419,000. Thinning would cost more on this day. The pilots nearest a limit have minutes left under 100 hours in 672.](the-noc.jpg)

Three hours before a weather system reaches the carrier's stations, the
operations centre projects what the system will cost under each of its 4
policies:

- **absorb**: fly the schedule and take the delays.
- **thin 15%** and **thin 30%**: cancel that share of the carrier's
  arrivals into the weather, in its window, now. The cheapest arrivals go
  first, each with the same aircraft's next departure out. The flights
  that still operate get the slots, and the cancellations are announced
  with every message a cancellation sends.
- **standby crews**: bring in 2 standby crews at each crew base under the
  weather. A standby crew is a set of volunteers on a day off, paid half
  again to sit reserve.

The operations centre then sends a decision to the seat that runs it,
with each policy's expected cost and its 95th percentile. The default is
the cheapest policy. The autopilot of a carrier that nobody runs takes no
part, unless the world runs with `skyd -noc-autopilot`. In that case
every carrier's operations centre projects and acts. That costs CPU in
proportion to the number of carriers that the weather touches.

The plan holds the true weather. The operations centre sees a forecast
of it. The onset is known to within a quarter of the lead time, the
duration to within three tenths, and the rate to within a tenth.

## Projections

A projection plays one carrier's day many times through the same plan and
roster as the world (`internal/noc`). Each play draws a value for every
uncertainty in the scenario. It then builds the day with that weather,
those ground stops, that sick rate and those aircraft gone technical. It
staffs the day with a copy of the carrier's roster and prices it. The
price of a play is the sum of these amounts:

- the revenue lost: refunds, and passengers with nothing to rebook onto,
- the compensation and care owed,
- the crews and the ground staff,
- a delay cost of 50 dollars a minute past 15 minutes for fuel, aircraft
  time and goodwill (the world's number).

Passengers of a cancelled flight go onto the carrier's next flight over
the same pair with seats free, the same day if possible, or the next
day. At the carrier's hubs, a share of the passengers on a late arrival
connect onward. None of them miss the connection at 30 minutes late, and
all of them miss it at 150 minutes late. A missed connection is
compensated on the delay at the final destination, as the Court of
Justice ruled in Folkerts (C-11/11). A hub here is any station that takes
1 in 12 of the carrier's arrivals.

The cost of the disruption is each play's price less the price of a calm
day. The projection reports the mean, the standard error, the 95 per cent
interval of the mean, the 5th, 10th, 50th, 90th and 95th percentiles, a
histogram, and the same statistics for every count: cancellations, delay
minutes, missed connections, crew cost and revenue lost.

The tools use these methods:

- **Monte Carlo** (`"method": "mc"`) draws every uncertainty
  independently for each play.
- **Latin hypercube sampling** (`"lhs"`, the default) divides each
  uncertainty's range into as many strata as there are plays, and draws
  each stratum once. With few plays, the error of the mean is smaller than
  with plain Monte Carlo for the same cost.
- **Common random numbers.** Every policy is played on the same draws. The
  difference between two policies is computed play by play, and its
  interval excludes the noise of the weather. A policy that saves 18,000
  dollars plus or minus 1,500 is better than absorbing the weather. The
  interval of its mean cost alone would not show that.
- **Sensitivity.** Each uncertainty is set alone to its 10th and 90th
  percentiles, with the others at their medians. The swings are sorted by
  size. This is a tornado diagram, and it shows which uncertainty the
  answer depends on most.
- **Stress cases.** Every uncertainty is set to the same quantile in its
  adverse direction: the event happens, runs long and cuts deep. P50 is
  the expected case, P90 the bad day and P99 the severe day.
- **Historical bootstrap.** When the world is a recorded day, or
  `skynoc -history` names one, each play draws every flight's own delay
  from the recorded day's delays. The background day then has the shape of
  a real one, and the disruption is measured against it.

An event has a probability. A PROB30 group in a forecast happens in 3
plays in 10.

### Scenarios

A scenario is a list of events in JSON. Each uncertain number is a
distribution:

```json
{"name": "storm over the hub", "events": [
  {"kind": "weather", "airport": "ORD", "probability": 0.7,
   "onset":    {"kind": "triangular", "a": 780, "b": 840, "c": 900},
   "duration": {"kind": "triangular", "a": 180, "b": 240, "c": 360},
   "factor":   {"kind": "triangular", "a": 0.35, "b": 0.45, "c": 0.6}},
  {"kind": "sickout", "onset": {"kind": "fixed", "a": 0}, "duration": {"kind": "fixed", "a": 1440},
   "rate": {"kind": "uniform", "a": 0.04, "b": 0.1}}]}
```

The event kinds are `weather` (the airport's arrival rate falls to
`factor` of normal), `groundstop` (departures to the airport are held),
`closure` (the airport is closed both ways), `sickout` (a share of the
carrier's crews call in sick) and `aog` (a number of the carrier's
aircraft go technical in the window). Times are minutes from the day's
midnight UTC. The distributions are `fixed`, `uniform`, `triangular`,
`normal` and `lognormal`.

### Real forecasts

A projection can take its scenario from the terminal aerodrome forecasts
(TAFs) that aviationweather.gov publishes. The service is the National
Weather Service's, is free, and covers stations worldwide. Each forecast
group that takes an airport's rate below 95 per cent becomes a weather
event:

- The group's flight category comes from the National Weather Service's
  definitions: LIFR below a 500-foot ceiling or 1 mile, IFR below 1,000
  feet or 3 miles, MVFR to 3,000 feet and 5 miles.
- The rate that a category leaves is the world's mapping: VFR 1, MVFR
  0.9, IFR 0.75, LIFR 0.6. Thunderstorms leave half, snow 0.6, freezing
  rain 0.45. Gusts of 35 knots take another fifth. Airports publish their
  own arrival rates by runway configuration, and this mapping does not
  use them.
- A PROB30 or PROB40 group happens with that probability.
- A TEMPO group lasts between a sixth and a half of its period, starting
  anywhere in it. The National Weather Service's TAF training (lesson
  4.4) says that each occurrence lasts an hour or less, and that together
  they cover less than half the period.
- A base or FM group holds for its period, and its onset is known to
  within half an hour.

`skyd -live-weather` reads the forecasts of the world's 40 busiest
airports every half hour. It adds the weather that they forecast, when
more likely than not, to the day as each forecast is issued. This is
meaningful at `-warp 1`, when the world's clock is the real one.

## Adding events

`POST /noc/inject` adds an event to the running day:

```sh
curl -X POST localhost:8080/noc/inject -d '{"kind":"weather","airport":"LHR","in":200,"duration":240,"factor":0.45}'
curl -X POST localhost:8080/noc/inject -d '{"kind":"sickout","carrier":"BA","rate":0.12}'
curl -X POST localhost:8080/noc/inject -d '{"kind":"groundstop","airport":"JFK","in":60,"duration":90}'
```

The plan slots the arrivals that are still on the ground, holds the
stopped departures, and works again every tail that it touches: late
aircraft, crews and reserves. Every carrier that the event touches gets an
incident on its tape. Every operations centre sees the event as a
forecast. On a multi-machine world, the core passes the event to every
machine. Anyone may add an event, as anyone may close an airport. Each
visitor may add one event per half-minute. The world's operator, with the
link secret, has no limit.

## The API and the tools

| Path | What it answers |
| --- | --- |
| `GET /carrier/<code>/crew` | the roster: headcount by base, fleet and position, the agreements, the costs so far, the pilots nearest a limit |
| `GET /carrier/<code>/crew/<id>` | one member: the last 28 days and the time left under each limit |
| `GET /carrier/<code>/noc` | the operations centre's signals and its last projection |
| `POST /carrier/<code>/noc/project` | a projection: `trials`, `method`, `policies`, `sensitivity`, and a `scenario`, real forecasts (`taf`) or `events` added to the forecast |
| `POST /noc/inject` | an event added to the day |
| `GET /noc/<code>` | the operations centre's page |

The levers gain `standby` (`board`, `count`), `noc` (`policy`) and
`respond` (`unit`, `choice`). The departments gain `noc` and `labour`.
`cmd/skyagent` has the tools `crew`, `crew_member`, `noc`, `project` and
`inject`.

`cmd/skynoc` runs a projection offline against a compiled world, without
a running sky:

```sh
go run ./cmd/skynoc -world world.json -carrier UA -wx ORD:1400:240:0.45 -sensitivity
go run ./cmd/skynoc -world world.json -carrier UA -taf -trials 1000
go run ./cmd/skynoc -world world.json -carrier DL -sickout 0.08 -groundstop ATL:1600:90
go run ./cmd/skynoc -world world.json -carrier UA -wx ORD:1400:240:0.45 -history thanksgiving.json
```

A weather flag is `AIRPORT:HHMM:MINUTES:RATE[:PROBABILITY]`. Without
`-certain`, each flag gets a forecast's uncertainty: 45 minutes either
side for the onset, three tenths of the duration, and a tenth of the rate.
On the whole sky's UA, 200 plays of 4 policies take about 10 s on a
laptop.

## What is not done

- Pairings across days. A duty away from a base is a layover crew from
  the hub, and the roster does not build the trip that brought it there.
- Deadheads to bring a reserve to an outstation.
- Monthly guarantees, line value and overtime above it.
- Crew regimes other than Part 117 and Part 121, starting with EASA
  ORO.FTL.
- Ground staff by name. Stations have staffing costs, not rosters.
- Maintenance crews, dispatchers and the operations centre's own staff.
- Arrival rates by runway configuration for the real forecasts.
- Recovery that swaps aircraft between tails, which a real operations
  centre does before it cancels.
