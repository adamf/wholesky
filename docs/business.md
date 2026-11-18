# The business model

This document describes the money side of a wholesky world: what a flight
costs, what a ticket sells for, who buys it, who is paid for it, and how
a carrier competes and grows. It names the source of each number.

The model has three kinds of number. A measured number is read from a
public data set that the repository holds. A published number is a
constant from a paper, a court record or a company report. An estimate
is a value that no public source gives. The text says which kind each
number is. [internal/refdata/SOURCES.md](../internal/refdata/SOURCES.md)
lists every data file with its publisher and the date it was read.
[research/](research/README.md) holds the research notes behind the
numbers.

The mechanisms that a production system needs are in jetway: the fare
engine, minimum connecting times, proration and inventory control. The
synthetic fares, travellers and carrier behaviour are in wholesky.

## Fuel

`internal/fuel` holds the price of jet fuel for each day of the world.

- The reference price is the US Gulf Coast spot price of jet fuel. The
  Energy Information Administration (EIA) publishes it for each trading
  day from April 1990. The repository holds the series.
- A world that starts inside the record uses the recorded prices. Start
  a world on 2 January 2008 with `skyd -economy-start 2008-01-02`. Fuel
  goes from $2.75 a gallon to $4.04 by July, to $4.81 in September, and
  to $1.26 by late December.
- After the record ends, a fitted process continues the price. The
  logarithm of the price reverts to $2.27 a gallon at a rate of 0.81 a
  year. Volatility is 36 % a year, with about 4 jumps a year of 13.5 %.
  A test fits these values again from the series.
- A station pays the reference averaged over 60 days, plus a regional
  differential, plus an into-plane charge. In March 2026 the spot price
  rose 63 % and the price that US carriers paid rose 33 %. The
  differentials outside the United States are estimates from 3 weeks of
  IATA's price monitor.
- A flight burns the fuel that EUROCONTROL's emissions calculator gives
  for its aircraft type and distance, over a route 5 % longer than the
  great circle.

A carrier can hedge with a swap, a cap or a collar. The simulator prices
a cap with the Black-76 formula. Carriers that hedge in the airline
industry start with a hedge in place. The scorecard shows the result of
the hedges as a cost line.

A network carrier adds a fuel surcharge to long international tickets
when the 60-day average is above $2.00 a gallon. The step is $40 a
sector on long haul and $17 on medium haul for each $10 a barrel. The
formula follows the surcharge tables that the Japanese carriers publish.
The baseline and the step sizes are estimates.

## Aircraft

`internal/equipment` holds what each aircraft costs.

| Item | Source | Kind |
| --- | --- | --- |
| Value and monthly rent by type, new and at 10 years | Appraisers' figures in the trade press, 2015 to 2026 | Published, with estimates marked by row |
| Maintenance per block hour | IATA's maintenance cost commentary for 2024, scaled by type | Estimate |
| Share of the fleet that is leased | About half of the world fleet. Low-cost carriers 60 % or more, large US carriers 20 to 35 % | Published |
| Security deposit | 1 to 3 months of rent. The model uses 2 | Published |
| Maintenance reserves | 44 % of rent at 300 flight hours a month, from one lessor's filing | Published, one example |

Each tail has an age, an owner and a rent. The simulator derives them
from the tail number, and every machine therefore agrees. The scorecard
charges rent or ownership cost for each hour of the day, and maintenance
for each block hour flown.

## Airports

`internal/airport` holds what an airport costs and what it constrains.

- **Charges.** The Jacobs review of airport charges for 2024 prices a
  landing and a take-off at 50 airports. An airport outside the review
  takes the level of its region, which is an estimate. A low-cost
  carrier pays 65 % of the published charge and an ultra-low-cost
  carrier 40 %. These 2 shares are estimates.
- **Navigation.** The en-route charge uses EUROCONTROL's formula: the
  distance times the square root of the weight over 50 tonnes, times a
  unit rate.
- **Slots.** A coordinated airport has a slot value by tier. The tiers
  follow recorded trades: Heathrow pairs at $60 million to $75 million,
  Washington National at $5 million to $6 million, LaGuardia at
  $2 million to $4 million, and Newark at $1.2 million.
- **Minimum connecting time.** A station takes its published standard
  where the reference table has it. Other stations take a default by
  size. jetway's `pkg/mct` reads the industry's file format when an
  operator supplies the licensed file. A connection is sold only when
  it meets the time. On the day, a passenger whose inbound flight is
  late misses the connection with a chance that rises as the delay
  uses up the time in hand. The curve is an estimate, shaped to the
  3 connecting passengers in 100 that a 2007 study found.

### Slots on the day

When a regulation limits arrivals, each flight gets a slot. The rules
differ by region.

- In Europe the simulator follows EUROCONTROL's slot allocation. A
  cancelled flight frees its slot, and the flights behind it move up in
  the order of their planned times. A flight that reports ready (REA)
  can take an earlier slot with 5 minutes of notice.
- In the United States and Canada the simulator follows the Federal
  Aviation Administration's ground delay program. The carrier that
  cancels keeps the slot for its own flights first. Other carriers'
  flights then move up.

Before this work a cancelled flight kept its slot for the whole day.

## The turnaround

`internal/equipment` builds a turnaround from its tasks. Passengers
leave, the cabin is cleaned, and passengers board. Bags come off and go
on. Fuel goes in. The slowest of the 3 paths sets the minimum time.

The task rates come from the terminal servicing charts in the
manufacturers' airport planning documents. Boeing's chart gives a
737-800 with 162 passengers through one door 37.5 minutes with full
service. Airbus's chart gives an A320 with 180 passengers, 2 stairs and
no fuel 22 minutes, and Ryanair schedules 25. A 777-300ER on a long
flight needs about 62 minutes at the minimum, and the schedule gives it
90 or more.

The schedule gives each aircraft the time for its type and its carrier's
model. A late inbound aircraft delays the next departure only when the
delay is longer than the slack in the turnaround.
`GET /carrier/XX/turn?flight=&board=` shows each task with its start and
end.

## Fares

jetway's `pkg/fare` prices an itinerary as Carl de Marcken describes in
"Computational Complexity of Air Travel Planning". A journey divides
into fare components. Components group into priceable units: one way,
round trip, open jaw and circle trip. Each fare has rules, and some
rules apply to the whole unit. The engine searches the divisions and
returns the lowest price with a fare calculation line. The tests use the
paper's own examples.

`internal/tariff` files the synthetic fares.

- **Level.** The Department of Transportation's Consumer Airfare Report
  gives the average fare in each of the 1,000 largest US city pairs. A
  straight line fits 4 quarters of it: $153.00 plus 7.74 cents a mile.
  Other regions scale that line by an estimate.
- **Ladder.** Each market has 14 booking classes. Full fare is 3.6 times
  the lowest regular fare, inside the range of 3.3 to 4.1 in Peter
  Belobaba's lectures. The cheaper classes need 3 to 30 days of advance
  purchase and carry change fees.
- **Carriers.** A network carrier files at 1.06 of the market level, a
  low-cost carrier at 0.80 and an ultra-low-cost carrier at 0.62.
- **Through fares.** A carrier files a through fare over its own
  connection or a partner's at 92 % of the 2 local fares.
- **Fences.** On long international markets, the 4 cheapest classes of a
  network carrier need a round trip with a Saturday night. One class
  sells only in the carrier's own channels. Corporate accounts get 15 %
  off 2 classes.
- **Taxes.** US taxes follow the 2026 rates. The UK Air Passenger Duty
  applies to departures from the United Kingdom.

### Calibration

A test fills a US world of 12 carriers and 12,237 flights and compares
the result with the report.

| Measure | Target | Result |
| --- | --- | --- |
| Load factor | 0.82 | 0.812 |
| Fare paid, as a share of the report's line | 1.00 | 1.006 |
| Ultra-low-cost carriers, as a share of their markets | 0.749 in the report | 0.72 |
| Travellers who chose a fare that the engine refused | 0 | 0 |

The test fails when a change to the ladder, the class limits or the
traveller model moves these results. The test output says which constant
to move.

## Travellers

`internal/market` holds the passenger choice model of the Passenger
Origin Destination Simulator (PODS). Boeing built PODS, and the
Massachusetts Institute of Technology has used it since 1994 to compare
revenue management systems. The constants are the published ones.

- A traveller is on business or on leisure. In a short market 30 % are
  on business, which is an estimate.
- Each traveller has a maximum price. Half of the leisure travellers
  pay 1.2 times the lowest fare. Half of the business travellers pay
  7.5 times the lowest fare.
- Each traveller puts a cost on each restriction. For a business
  traveller a Saturday night costs 2.25 times the lowest fare. A fare
  with no refund costs 0.75 times, and so does a change fee. For a
  leisure traveller the 3 costs are 1.75, 0.25 and 0.25.
- A connection, each hour away from the wanted departure, and a booking
  outside the traveller's own channel also have costs. These 3 costs are
  estimates.
- A traveller in a carrier's loyalty programme values that carrier at
  15 % of the lowest fare, and an alliance partner at half of that.
  Both values are estimates.
- Leisure travellers book early. Half of the business travellers book
  in the last 14 days.

A traveller removes the offers above the maximum price and takes the
offer with the lowest total cost.

The live demand uses this model. A traveller wants a trip between 2
airports. The simulator lists each carrier's nonstop flights and each
connection that meets the minimum connecting time. The fare engine
prices each offer at the lowest fare that the rules, the channel and the
open classes allow, as of the traveller's own booking date. The booking
carries that quote. In the test world a carrier that cuts its fares 20 %
goes from 308 sales to 377. A carrier that raises them 25 % goes to 235.

The advance sales use the same model. For each flight the simulator
draws a pool of travellers and sells to them in the order of their
booking dates.

## Revenue management

Each carrier's inventory nests its booking classes. An EMSR-b controller
sets the class limits from a forecast of demand by class. The forecast
reads the booking curve on the world's clock. For a connecting request
the controller compares the fare with the bid prices of the legs.

The forecast's class mix is the measured mix of the calibration test.
About 1 traveller in 9 pays full fare. More than 1 in 3 fly in the 4
cheapest classes. About 1 in 30 flies in the front cabin.

A network carrier's narrow-body has a front cabin with 2 seats on each
side of the aisle: 12 seats on an A320, 16 on a 737-800 and 20 on an
A321. The large US carriers fit about that. A low-cost carrier's
narrow-body has one cabin. In a short market 7 % of travellers want the
front cabin, which is an estimate set to the size of the cabin. The
fares of the front cabin are 1.11 to 1.67 times full fare, and those
ratios are also estimates.

jetway's `inventory.Controller.Value` lets a carrier value a request net
of the cost of the channel that asked. A carrier that holds its last
seats for its own channels uses it. See the next section.

Travellers buy only the flights that have not left. A flight stops
selling 45 minutes before departure. The carrier's availability messages
name 4 classes of each flight every 30 seconds, and each other class of
the day's flights when its status changes.

Two faults in the seat control came out of this work. Both are fixed in
jetway. A class that had sold past a limit which the controller then
lowered was counted as having no limit, and it sold to the last seat of
the cabin (v0.1.101). A carrier's node could not sell its own flight
(v0.1.100).

## Channels and what a sale costs

`internal/distribution` holds the sellers and the cost of a sale in each
channel. A traveller books in one of 4 channels: the carrier's own site,
an online travel agency (OTA), a travel management company (TMC) or a
travel agent.

### Sellers

A world has 16 agencies: 6 OTAs, 3 TMCs and the travel agents of each
of 7 regions. The agencies are invented, and their names belong to no
company. Each agency has a number in the form of an IATA agency number
and books through one distribution system. `GET /agencies.json` shows
each agency with what it sold and what it earned.

A booking takes one of 3 paths.

- **The carrier's own site.** The booking is made on the carrier's own
  node. The seat comes from the carrier's inventory at once, and no
  message leaves the node. No distribution system is paid. jetway
  v0.1.100 added this path: before it, a carrier's node could not sell
  its own flight.
- **An agency through its distribution system.** The system sends the
  sell to the carrier. The record carries the agency's number. The
  carrier pays the booking fee, and the system passes half of the fee to
  the agency as an incentive.
- **An agency over a direct connection.** 3 of the agencies have a
  direct connection (NDC) to the carriers that offer one. Half of their
  sales on those carriers go to the carrier's own node, with the
  agency's number on the order. The share is Lufthansa Group's figure
  for the middle of 2026, used for every carrier, and is an estimate.

A carrier's own node sells the itineraries that are under its own code.
An itinerary with another carrier's flight goes through a distribution
system.

An agency earns from 3 sources: the incentive from its distribution
system, the commission or override from the carrier, and its own fee
from the traveller. The large OTAs charge the traveller nothing, a
regional OTA charges $5, and a travel agent charges $25. A TMC charges
the traveller's employer $15 for each transaction. A US commission on
travel distribution heard in 2002 that travellers paid agents' fees up
to $25. The other fees are estimates.

### Costs

| Cost | Value | Source |
| --- | --- | --- |
| Booking fee for each segment, North America, with a full-content agreement | $2.30 | The weighted average of American, Delta and United at Sabre in June 2019, from United States v. Sabre |
| The same without the agreement | $3.40 | US Airways' contract of 2011, from US Airways v. Sabre |
| Booking fee for each segment elsewhere | $4.00 at home, $6.00 abroad | Estimates from Amadeus's revenue of 5.90 euros a booking in 2023 |
| Card and payment cost | 2 % of the amount paid | IATA's figure for 2025 is 1.97 % |
| A sale on the carrier's own site | $1.50 | Estimate |
| An agency's sale over a direct connection | 15 % of the booking fee, and $2.00 to the agency | Farelogix charged about a tenth of Sabre's fee, and American paid agencies $2.00 from 2017. Both are in the record of United States v. Sabre |
| Incentive that a distribution system pays the agency | 50 % of the booking fee | Sabre's cost of revenue was $2.93 to $3.11 a booking in 2022 to 2024. Amadeus's was 51 % of its air distribution revenue in 2023 |
| Agency commission | None in North America, Europe and Oceania. 3 % elsewhere | Published range of 1 to 5 % |
| Override paid by a network carrier | 2 % to a TMC, 1 % to an OTA | Estimates |

The OTA does not keep a share of the fare in most markets. The
distribution system collects the booking fee from the carrier and passes
about half to the agency. In the test world, distribution costs a
network carrier 2.7 % of its fares and card fees cost 2.0 %.

A carrier has a channel policy. The action `channel` changes it.

- **Leave a channel.** An ultra-low-cost carrier sells only on its own
  site. Of the travellers who shop elsewhere, 2 in 3 find the site and
  1 in 3 are lost. The split is an estimate.
- **Surcharge a channel.** Lufthansa added 16 euros to agency tickets in
  2015. Its direct share rose from 30 % to 45 % in 3 years.
- **Keep the lowest fares for the carrier's own channels.** American
  tried this in 2023 and reversed it in 2024.
- **Pay an override.** A carrier that pays more appears higher on the
  agent's display.
- **Offer a direct connection.** Network and low-cost carriers do by
  default.
- **Hold the last seats for the carrier's own channels.** The carrier
  values a request from a distribution system at the fare less the
  booking fee. It sells a class through a distribution system only
  while the class below it is open, and it tells the distribution
  systems that status. Its own site sells the class to the limit. A
  full-content agreement forbids this, and a carrier that turns it on
  gives the agreement up. Low-cost carriers do this by default.

In the filled US test world 53 % of tickets are direct, 23 % come from
OTAs, 16 % from TMCs and 8 % from agents. US carriers report direct
shares of 55 to 65 %.

## Alliances, codeshares and who gets what

`internal/partners` holds the 3 alliances from their member lists, and
the joint ventures from the US and European immunity decisions. Two
carriers have one of 4 ties.

| Tie | Service charge on a coupon that the partner flies | Source |
| --- | --- | --- |
| Interline only | 9 % | The default of the multilateral agreement |
| Alliance | 6 % | Inside the usual range of 5 to 7 % for a special prorate agreement |
| Codeshare | 7 % commission | Estimate |
| Joint venture | 0 % | The partners pool the revenue of the routes |

When one carrier flies a coupon on another carrier's ticket, the money
divides in 2 steps.

1. The fare engine divides each through fare between its segments by
   their miles. This is straight-rate proration. A segment with a fare
   of its own keeps that fare.
2. The carrier that flew the coupon bills the ticketing carrier for the
   coupon's share. The ticketing carrier keeps the service charge.

An example from the tests: United tickets Chicago to Athens on a through
fare of $880 and flies the first leg. Lufthansa flies the second leg.
The legs are 7,000 km and 1,800 km, and the fare divides into $700 and
$180. Lufthansa and United are in a joint venture, and Lufthansa
therefore bills the full $180. A carrier with only an interline
agreement bills $163.80 for the same coupon.

`/billing.json` shows each invoice with its tie. The scorecard shows
what a carrier kept from partners and what it paid them.

A joint venture also pools what its routes earn. The world has 7
ventures, and each has a scope. For example, United, Air Canada and the
Lufthansa group cover the flights between North America and Europe.

1. Each party puts the revenue of its own flights on those routes into
   the pool.
2. Each party takes out a share of the pool. The share is the party's
   part of the seat-kilometres that the parties flew on the routes.
3. A party that earned more than its share pays the difference. A party
   that earned less receives it.

A party therefore gains nothing when it takes a passenger from a
partner. This is the purpose of a pool
([Brueckner and Singer, 2019](research/alliances.md)).

An example from the tests: United and Lufthansa both fly between Chicago
and Frankfurt with the same aircraft. United earns $600,000 on 3 flights
and Lufthansa earns $200,000 on 2 flights. United has 60 % of the seats
and 75 % of the revenue. Its share of the $800,000 pool is $480,000, and
it pays Lufthansa $120,000.

The scorecard shows the amount as `joint_venture`, and `/ventures.json`
shows each pool. The real formulas are not public, and the capacity
share is an estimate. The European Commission describes SkyTeam's
Atlantic venture as a share of profit. The model shares revenue in all
7 ventures.

An alliance changes 3 other things. Partners file through fares over
each other's flights. A traveller values a partner of the traveller's
own programme. A cancelled passenger moves first to the carrier's own
flights, then to a joint venture partner, then to a codeshare partner,
then to the alliance, then to any carrier.

## Fare competition

`internal/pricing` holds a pricing desk for each carrier. Fares are
filed 4 times a day. At each filing every desk sees the changes that
other carriers made since the last filing.

| Rule | Value | Source |
| --- | --- | --- |
| A carrier matches a cut by a carrier of its own kind and does not undercut | 95 % of the time | The record of United States v. Airline Tariff Publishing Co. The rate is an estimate |
| A network carrier ignores a low-cost carrier | Under 10 % of the seats | Spirit v. Northwest: one daily flight got no response for 6 months |
| A network carrier matches a low-cost carrier in full | 25 % of the seats or more | Estimate |
| A low-cost carrier moves only when a rival's fare goes under its own | | Estimate |
| A carrier follows an increase | Network 85 %, low-cost 45 %, ultra-low-cost 10 % | Estimates set to make about half of all increases hold, as FareCompare counted for 2007 to 2015 |
| An increase comes back down | When a carrier with 10 % of the seats has not followed after 8 filings | Published practice. The threshold is an estimate |
| A network carrier attacked at its hub also cuts in the attacker's hub markets | Up to 3 markets | The Justice Department's complaint describes more than 50 such exchanges |
| The answers are withdrawn | When the first cut is withdrawn | Published practice |

A carrier that a person prices by hand does not move. The desk sends its
move to the seat as a decision, with the desk's move as the default.
Each filed fare goes to every machine of the world as an event, and the
state file keeps it.

Before this work a carrier on autopilot never changed a fare, and a
competitor's cut was a number that the simulator invented.

The advance sales also respond. A carrier with fares above its rivals'
gets fewer travellers in its pool, with an elasticity of 0.8. The
studies that InterVISTAS collected for IATA in 2007 put the elasticity
of a route near 1.4 and that of a national market near 0.8. The value
used is an estimate inside that range.

## Growth

`internal/growth` holds a carrier's money across days and what a new
service would do.

**The book.** When a day ends, the simulator closes each carrier's
scorecard into its book. Profit goes to cash. The book reports margin,
load factor, revenue and cost for each seat-kilometre, yield, and the
days of cash left. Opening cash is 18 % of a year's revenue at the
filed schedule. The 4 largest US carriers ended 2019 with 13 to 23 %.
`GET /carrier/XX/finance` shows the book.

**The projection.** `GET /carrier/XX/growth?from=&to=&type=&daily=`
projects a service of daily round trips on one aircraft type.

| Step | Value | Source |
| --- | --- | --- |
| Market size | Passengers a day in the Consumer Airfare Report, where the report has the airport pair | Measured |
| Market size elsewhere | Seats flown at a load factor of 0.82, with 3 in 4 local | Estimate |
| Share of the market | Flights to the power 1.2, and fare level to the power of minus 0.8 | Wei and Hansen (2005). Button and Drexler (2005) found a weaker curve |
| A low-cost entrant enlarges the market | 28 % | Beckenstein and Campbell (2017), 109 markets |
| Network carriers cut fares at entry | 21 % for a low-cost carrier, 15 % for an ultra-low-cost carrier | Goolsbee and Syverson (2008). Brueckner, Lee and Singer (2013) |
| A new route fills | From 72 % of its mature load, over 180 days | Spirit's Detroit to Philadelphia flight in 1996 |
| Aircraft needed | 10.8 block hours a day for a network carrier, 11.2 low-cost, 11.9 ultra-low-cost, 13.0 wide-body | Form 41 for 2019 |
| Costs | The scorecard's own figures for fuel, maintenance, airports, navigation, handling, crew, rent and selling | As above |
| Cash to start | 2 months of rent for each aircraft, and the price of slots at a coordinated airport | As above |

The projection returns the load factor at maturity, the break-even load
factor, the first 180 days, the days to pay back the start, and notes on
each assumption.

**The order.** The action `grow` places the order. The cash to start
leaves the book at once. The service comes due in 42 days, or in 90
days at a station that the carrier does not serve. A schedule change in
the airline industry takes 2 to 10 weeks.

**The schedule change.** A schedule changes overnight. A world on one
machine ends the day on which an order comes due, stops, and starts
again with the service in its schedule. The new world reads the state
file, which holds the calendar, the books, the fares and the policies.
The clock follows the wall clock, and the new world is at the same
minute of the day. A world on several machines does not do this. It
needs one world file for all of its machines. Write that file with
`worldc -in world.json -grow state.json -o next.json`.

**The answer to an entrant.** At the end of each day, a carrier looks
for a rival that has begun a service on a route from the carrier's hub.
The carrier answers a weak rival with flights.

| Step | Value | Source |
| --- | --- | --- |
| A weak rival | Fewer than 10 aircraft, or cash for less than 12 months of its losses | [Rule 6 of the competition note](research/competition.md) |
| Flights added | Half as many again as the carrier flies on the route | American added 35 to 80 % against Vanguard and Western Pacific in 1995 and 1996 |
| Limit | 2 round trips for each of the rival's | Qantas against Virgin Australia, 2011 to 2014 |
| Condition | The added flights earn at least their cost without the aircraft rent | The court in *United States v. AMR* found no liability inside this line |
| Time | The order comes due in 42 days | The record is 1 to 3 months |
| A rival that is not weak | No flights added. The pricing desk answers on price | American against Southwest. Goolsbee and Syverson (2008) |

A carrier answers each new service once, and an answer is not itself
answered. The order records the rival's order that it answers. A seat
that takes the `network` department decides for itself.

## What is missing

- **Direct sales on several machines.** A sale on the carrier's own
  site is made on the carrier's node only when the machine that takes
  the traveller also runs the carrier. On other machines it goes through
  a distribution system, and the carrier sees it as an agency sale.
- **Agencies as players.** An agency has an account and no decisions. It
  does not set its fee, choose a distribution system or bargain with a
  carrier.
- **Premium economy and upgrades.** An aircraft has at most 2 cabins.
  Nobody is upgraded into the front cabin, and its fares are estimates.
- **Joint ventures as one seller.** The parties of a joint venture pool
  revenue. Their pricing desks still compete with each other, and no
  venture shares profit.
- **A rolling calendar.** The world flies one date, and flies it again
  each day. At the end of a day the books are emptied and written again.
  A traveller's booking date is a property of the traveller, and a
  return flight is on a later date that the world never flies. A
  calendar that rolls needs a book for each future date, filled to its
  place on the booking curve. The next date's book must then become the
  day's book. On the recorded day, each date is 1.4 million records.
- **After a missed connection.** A passenger whose inbound flight is
  late misses the connection by the curve in `internal/airport`, and the
  scorecard counts them. Nobody rebooks them, and they cost the carrier
  nothing.
- **Ancillary revenue.** A ticket has a fare and taxes. Bags, seats and
  other fees are not sold. The fares are calibrated to a report that
  excludes fees. US carriers earned 4.6 % more than their fares from
  bags and changes in 2023. Spirit earned $53.01 of fare and $68.57 of
  other revenue for each passenger segment. A model with fees needs a
  lower base fare for the ultra-low-cost carriers, the fee in the price
  that a traveller compares, and a new calibration. The
  [research note](research/rm-fares.md) has the figures.
- **Exits.** A carrier does not drop a service, leave a market or fail.
  An incumbent therefore never takes its added flights out again, and
  fares do not return after a fight.
- **Schedule change on several machines.** Only a world on one machine
  starts again on its own when an order comes due.
