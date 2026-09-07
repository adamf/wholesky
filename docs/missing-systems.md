# The systems that are not here yet

wholesky runs the passenger side of a day of aviation. This covers
reservations at the carriers, sales at the distribution systems, and
departure control and sortation at the airports. The datalink and air
traffic services (ATS) networks run beside them. Seat inventory, schedule
changes and reaccommodation are also modelled. These are the core systems
of an airline's IT. The model
is now complete enough that the missing systems are visible. This
document lists the systems a full airline runs that the world does not
model. For each system it says what the system would add and what it
would exercise in jetway. The list is ordered by each system's share of
the day's traffic and by how much of that traffic jetway could carry
today.

## Large systems on the wire

**Fares and pricing (ATPCO, fare rules, taxes).** Done. Every booking is
priced before the sell against a fare filing derived from the schedule
(jetway `pkg/fare`). The filing has a ladder of classes per market with
fare basis codes, advance-purchase and stay rules, change fees and
refundability, and taxes shaped like a US domestic ticket. The Airline
Tariff Publishing Company (ATPCO) filing itself is licensed, so the
structure is the industry's and the amounts are synthetic.

**Ticketing and settlement (BSP/ARC, interline billing).** The Billing
and Settlement Plan (BSP) side is done, in jetway v0.1.72 `pkg/bsp` and
wholesky `internal/settle`. It is specified against IATA's public DISH 23
handbook. A settlement plan runs at boot and at every day wrap over what
the distribution systems ticketed. It gives each airline its HOT file and
reconciles the file against the carrier's own book. The HOT file has
headers, a transaction per document with amounts, commission, coupons,
passenger and payment, and office and file totals. Amounts are over-punch
signed. Teletype carriers receive their ticket numbers as SSR TKNE
(v0.1.73), and the books then agree. The files are served at
`/settlement/<carrier>.hot`. The globe's money bar shows the day's gross.
On the multi-machine demo each region and distribution system settles the
books it holds, and the core merges the views. Where the agent's book is
on another machine, the carrier's copy is the report, and the plan counts
it as unverified. The numbers carry a caveat. The carriers' in-memory
books are bounded by `-max-records`. On the recorded day, a large
carrier's older records are therefore evicted before the plan runs. Those
records show as documents that the agent reported and that the carrier
does not hold. The cause is eviction and not a wire fault. Interline
billing is done as far as the method goes, in jetway v0.1.77
`pkg/prorate` and wholesky `internal/interline`. Every ticketed coupon
flown by one carrier on another carrier's document is prorated by mileage
across the ticket's coupons. On the recorded day that is 7,000
codeshares. The proration is straight rate, which is the arithmetic the
prorate manuals begin from. The operating carrier invoices the ticketing
carrier the coupon's share less a 9 per cent interline service charge.
There is 1 invoice per pair of carriers. The invoices are served at
`/billing.json` and `/billing/<biller>-<billed>.json`, and merged across
machines like the settlement. This is not IATA's IS-XML, because the
passenger record structures are behind the SIS participation guide.
Refunds are done (jetway v0.1.78). When the traveller cancels a ticketed
booking, the booking is refunded first. Open coupons go back, and used
coupons stay used. The plan reports the document again as a refund with
every amount reversed, dated when the money went back. The commission
therefore comes back with the refund. Exchanges are done (jetway
v0.1.80). When the IROPS engine reprotects a passenger, the ticket is
reissued over the new itinerary, and the old document's open coupons are
marked exchanged. The plan reports the new document with the original
issue behind it (BKS46) and the old document's value as its form of
payment (BKP84 EX). The plan also writes the RET file that the agents
would send it (jetway v0.1.82, chapter 5 of the same handbook). The file
is served with `GET /ret/<gds>.txt` on any machine that holds a
distribution system's book. The plan here still reads the books directly
and not the file. Agency memos are done (jetway v0.1.83). A
carrier's own copy of a record can price the passenger differently from
what the agent reported. The carrier then debits the shortfall with an
ADM or credits the excess with an ACM. Each memo names the document it
corrects in BKS45, as the handbook lays it out. The statement counts the
memos and their net. The memos carry a caveat. The world's agents and carriers
price from the same tariff, so memos are rare by construction. A machine
that holds only one side raises no memos. These parts are not done: net
reporting and card data, the Prorate Manual's provisos, and rejections
and billing memos between the carriers. The provisos are minima, factors
and special agreements.

**Revenue management.** Revenue management is done as far as the method
goes (jetway v0.1.67). `pkg/inventory` sells under nested class
authorisations. An EMSR-b controller sets the authorisations from a
demand forecast by class, and re-optimises them on every question. EMSR-b
is Belobaba's heuristic, the textbook method. Every carrier in the world
now runs it with the tariff's forecast. The forecast uses the class mix
the demand draws from, with total demand a tenth above the cabin. The
deep discounts therefore close while full fare sells to the last seat.
The forecaster now reads the booking curve with additive pickup. The
forecast for a class is what the class has sold plus the share of its
baseline demand still to come. The time to departure is measured on the
world's clock. A
flight selling behind its curve reopens its discounts during the day. A
flight ahead of its curve protects harder. Network control is on too
(jetway v0.1.81). The ladders give each leg a bid price, which is the
fare of the cheapest class still open, or the displacement cost. A
connecting itinerary on a single carrier is accepted only when its
through fare covers the sum of the legs' bid prices. This is the additive
leg bid-price heuristic. The network linear programme is on top of the
heuristic (jetway v0.1.83-84). Every 30 seconds each carrier solves the
deterministic programme over each point its book connects through. The
programme assigns seats to itineraries by fare within the legs departing
in the next 4 hours, within each product's demand still to come. The
demand to come is the forecaster's by class for the local passengers, and
the booking curve's for the connecting paths sold. The duals of the leg
capacities replace the ladders' displacement costs as bid prices. A leg
the programme did not reach falls back to its ladder. The solver is a
plain dense simplex. A connecting point is therefore bounded at 120 legs,
and the most-booked paths are kept. A forecast learned from history, in
place of the demand model's own mix, is still missing.

**Interline through check-in (IATCI).** IATCI is done (jetway v0.1.61-63,
`pkg/iatci`). Of the connections, 1 in 4 crosses carriers. The first
carrier's departure control asks the second carrier's departure control
over the switch with a DCQCKI. The second carrier seats the passenger on
its own flight and answers with a DCRCKA. The seat travels with the
connection. The structures are the PADIS release 01.1 layouts as publicly
mirrored. The members-only implementation guide was not consulted.

**Schedules distribution (SSIM, ASM/SSM in full).** Schedules distribution
is partly done. An aircraft going technical is modelled (jetway v0.1.65).
Of the flights, 1 in 150 is substituted after check-in opens. Departure
control rebuilds the cabin, then re-seats passengers or denies boarding.
The inventory shrinks to the new aircraft. The distribution systems
receive the ASM EQT and queue the bookings. Time changes are done too
(jetway v0.1.66). A flight running 46 minutes or more late is announced
about 2 hours out as an ASM TIM. The distribution systems move the sold
segments to the new times at TK and queue the advice. The panel shows
STD → ETD. The SSIM file is now a source too (jetway v0.1.68). `pkg/ssim`
writes chapter 7 records to the layout that 2 open-source parsers share.
`worldc -ssim` writes the compiled day as 1 carrier file after another,
with codeshares as DEI 050/010 segment data. `skyd -ssim` takes the day's
flights from such a file in place of the manifest's flights. A carrier's
own schedule file can therefore fly. The compiler still supplies
rotations, delays and the recorded day, because no schedule file carries
them.

**Baggage reconciliation and tracing (BRS, WorldTracer).** Reconciliation
is done (jetway v0.1.59). At the door, every bag the hold reported loaded
is matched to a boarded passenger. Unaccompanied bags hold the door until
sortation is told to pull them. Short-shipped bags are named for the
rush. The panel shows the count. The rush is done too. A short-shipped
bag follows on the carrier's next flight over the sector. Sortation is
told to load it. The arrival station's bag office is told to expect a bag
without its passenger, with a BUM (jetway v0.1.79). The panel says which
flight the bag rode. Tracing files are done as a profile (jetway
v0.1.83). When the passengers' flight lands without their bags, the
arrival station's tracing desk opens an AHL per bag. When the rush flight
lands, the bags come off alone. The desk raises an OHD for each bag and
matches it to the open file on the tag. The desk then sends the FWD that
delivers the bag, and closes the file. The panel shows the sequence per
departure. The
bag office's counters run on the carrier. WorldTracer is SITA's system,
and its formats belong to the vendor. The element codes here are the ones
that handler training material reproduces, and unknown elements are kept
verbatim. This is therefore the world's profile of a tracing file and not
the vendor's format. Nothing here loses a bag beyond the next flight.

**Border and government (APIS, PNRGOV, Secure Flight, timatic).** APIS is
done (jetway v0.1.64, `pkg/paxlst`). It is specified against the public
WCO/IATA/ICAO guide and tested on the guide's worked examples. Every
international door close sends the border control agency the flight-close
passenger list. The agency counts travellers by arrival. PNRGOV is done
(jetway v0.1.66, `pkg/padis`). It is specified against the free IATA
PADIS PNRGOV guide and tested on the guide's worked example. Every
international flight pushes its records to the state once when the name
list goes out. It pushes them again at the door with each traveller's
seat, sequence and bags. Secure Flight-style vetting before a boarding pass and timatic
are not done.

## Operations systems mostly off the wire

**Crew.** Crew is done as far as legality goes (jetway v0.1.85 `pkg/crew`,
wholesky `internal/dayplan`). 14 CFR Part 117's Tables A and B, the
2-hour extension and the 10-hour rest are transcribed and tested value by
value. Every tail's rotation is cut into duties as scheduled. A crew
flies as many legs as the table allows, then a fresh crew takes the
aircraft. Each duty is checked again with the day's delays. A duty that
fits only on the extension is noted on the panel. A duty that does not
fit is rescued by a reserve crew when the leg leaves the carrier's base,
with 90 minutes more delay. When the leg does not leave the base, the
flight is cancelled for crew, code A. On the recorded day the duties are
reported, and no new flight is cancelled. These parts are not done:
pairings across days, deadheads, rest at outstations, and cabin crew as
distinct from flight crew. Any regime other than Part 117 is also not
done. The rules are a value, and EASA ORO.FTL would be another table.

**Maintenance and aircraft rotation.** Tails are rotated through the
schedule. One flight in 150 goes technical after check-in opens. A smaller
aircraft takes the flight, the cabin is re-seated, the overflow is denied
boarding by name, and distribution receives an equipment change ASM (see
Schedules). What is missing is a maintenance programme: scheduled checks
that remove a tail from the rotation, and unserviceability that depends on
the aircraft's history.

**Flight planning and dispatch.** The ops desk files a synthetic route.
In the industry, dispatch computes the route, fuel and alternates from
weather and performance. Dispatch produces the operational flight plan
that the crew signs, and the loadsheet's fuel figure comes from that
plan. The link to air traffic control (ATC) exists, as `pkg/ats` over
`pkg/aftn`. The planning does not exist.

**Weather and air traffic flow.** Weather and air traffic flow are done
in the European form (jetway v0.1.85 `pkg/atfm`, wholesky
`internal/dayplan`). The synthetic day draws 2 to 4 weather systems from
the date. Each system is a few hundred kilometres across and sits over a
busy airport for 3 to 6 hours. It cuts arrival rates to between a half
and three quarters. The Network Manager regulates every airport under a
weather system. Arrivals in the window are held to the reduced rate,
first come first served. Each held flight gets a slot 2 hours before
off-block. The slot is a SAM with the calculated take-off time, the
regulation's name and the cause, WA 84 for weather at destination. The
SAM is in ADEXP to EUROCONTROL's public ATFCM Users Manual and tested
against the manual's own examples. The carrier's operations centre
receives the slot through jetway's Ground seam, and the panel shows the
CTOT. A late aircraft's delay passes to its next leg after the turn, as
late aircraft, code 93. The day's delays therefore chain. On the recorded
day, the record's NAS and weather minutes become the slots that explain
them. The globe shows the weather cells in force. These parts are not
done: the FAA's CDM messages and ground stops, the slot swapping and
improvement dialogue, and en-route regulations. Weather that closes an
area outright instead of slowing it is also not done. SMM, REA and SIP
are parsed and built, but no component sends them yet.

**Airport systems.** These are common-use check-in (CUPPS), gate
management, FIDS, stand allocation and de-icing. Today the airport is a
set of addresses. Gate and stand allocation matter for connections,
because a bag makes or misses its connection depending on the stand
distance. CUPPS is how a departure control system (DCS) is driven at a
shared airport.

**Ground handling and catering.** Ground handlers receive the load
messages the DCS already sends. The handlers, the turnaround times and
the catering that follows the passenger counts are not modelled.

**Cargo.** A passenger aircraft's belly earns cargo revenue. Air waybills,
Cargo-IMP and Cargo-XML messages, ULD control and the customs manifest
form a second messaging network the size of the passenger network. jetway
would parse Cargo-IMP as it parses Type B. Nothing has been started.

## Other distribution channels

**NDC and ONE Order.** A slice of demand arrives as NDC orders today.
Distribution is moving to a full NDC seller with offers and orders. It is
also moving to an order management system that replaces the passenger
name record (PNR) and the ticket. The 2 models must be reconciled during
the long transition.

**Direct channels and loyalty.** These are the carrier's own website and
app, and its loyalty programme with accrual, redemption and tiers. They
also include the customer identity that ties bookings across records.
Frequent flyer
numbers are on the filled records. Nothing reads them.

**Payments and fraud.** Nothing is paid for. Card authorisation at
booking, fraud screening, refunds and chargebacks are the systems that
decide whether a booking is genuine.

**Disruption communication.** Passengers are rebooked, and nobody is
told. Notifications, self-service rebooking, vouchers and compensation
(EU261) are the passenger-facing half of IROPS.

## The network

**Type B over IP in the forms the carriers receive.** SITA and ARINC
deliver Type B over MQ, over IBM MQ queues, and as email gateways. Over
HTTPS they deliver it as Type B over IP with acknowledgements. They also
deliver it over MATIP, which the world already speaks. A production
jetway needs the MQ and HTTPS deliveries.

**2 switches.** The second switch is done (jetway v0.1.69, wholesky
`-switches 2`). jetway's node gained the `link_dial` egress, which is a
dialled, bidirectional link that 1 switch holds open to another. `via`
routing sends the other switch's subscribers down that link. The world
runs 2 switches joined by that trunk. Every carrier is homed on 1 switch
by hash. The distribution systems, networks and border agencies are on
the first switch. A booking on a carrier across the trunk sells and
settles. A message for 1 local and 1 remote subscriber reaches each once,
and the trunk never carries it back to its origin. This is still a single
provider's view. Carriers hold a circuit to 1 switch and not to both. The
provider routing tables are configuration and not messages.

## The game and joined worlds

Running a carrier (docs/run-a-carrier.md) still lacks these items, in the
order they should be done:

1. ~~Persistence.~~ Done. Seats, claims, nodes and joined worlds survive a
   restart. The core keeps a state file, and the regions keep their state
   at the core.
2. ~~Consequences that cascade.~~ Done. A seat's cancellation, retime or
   reserve callout recomputes the tail's day, including late aircraft and
   crews. Every other change lands on the tape.
3. ~~Joined worlds in operation.~~ Done. wholesky-mirror.fly.dev is
   trunked to the demo, and the gate joins a mirror to the small world.
4. **More decisions.** Partly done. The pricing desk now receives the
   competition's fare changes, with a match-or-hold decision and
   travellers who shop. The tape carries weather, slots, crew timeouts and
   aircraft going technical as incidents. There are still no incidents
   that escalate or compound, no schedule-planning decisions and no
   seasons.
5. **Identity.** Anyone can take any free carrier. The token is the only
   protection. There are no accounts, no history across days and no
   season.
6. **The scorecard's shape.** Costs are chosen constants, and the
   composite is arbitrary. An external carrier's board lacks its own bags
   and denied boardings.
7. **The external node's autopilot** accepts every passenger with 1 bag
   and closes the flight. It has no connections, no no-shows, no ADL and
   no IROPS on the player's side. The ops page does not show the weather
   that affects your carrier. The MCP server has no event push.
8. **Federation as a single protocol.** Register, token, world, revenue
   and distribution are separate ad hoc endpoints without a version. They
   are now behind a single secret (docs/security.md).
9. ~~Recording and replay.~~ Done. Every seat's run is recorded from take
   to release, with the simulated clock on every line. The recording plays
   back at `/replay/<id>`. An agent narrates its own run with the `note`
   tool. The first recorded day is on the site.
10. ~~A security pass.~~ Done for everything that needed no decision. The
    pass left 7 decisions, which are in docs/security.md. The decisions
    are TLS on the trunks, invite-only joins, a public console that can
    book, and anonymous weather. They also include signed Type B origin,
    a relay hop count, and where seats live.
11. **A world that stays up.** `deploy/k8s` is the demo's shape on
    Kubernetes, with an Envoy edge that gives the switch ports raw TCP. It
    has not yet run on a cluster. Fly's shared IPv4 carries HTTP only, and
    for that reason the mirror world's trunk to the demo flaps there.

The recorded run found these bugs, which are not yet fixed:

- A world booted with `-fill` cannot refill after the day wraps. The
  end-of-day purge leaves the fill's own records, and the refill hits
  duplicate locators (`fill A5: store: duplicate`). The Thanksgiving world
  runs with `-fill 0.85`.
- The fill writes about 11 flights a second. A 5,364-flight world takes
  8 minutes, and the clock must be held for that time.
- The lobby ranks a carrier's score against the world, and the ops centre
  shows the raw score. The same carrier therefore reads 139.6 in the lobby
  and 89.6 in the ops centre.
- A world booted mid-day shows the flights that have already departed with
  the demand's bookings alone. Their load factor therefore reads near zero
  until the filled flights fly.

## Order of work

Fares, revenue management, IATCI and APIS are done. The next items, in
order, are:

1. Weather systems that close regions, not only airports, with the
   Network Manager's regulations that follow.
2. Booking curves with seasonality, so a day in August differs from a day
   in February.
3. Filling a recorded day to its measured passenger load, which needs the
   books on a database (`-tenant-dsn`) and a faster fill.
4. Crew regimes other than 14 CFR Part 117, starting with EASA ORO.FTL.
5. Identity for seats: accounts, history across days, a season.

Cargo is a second project with the same shape as the first.
