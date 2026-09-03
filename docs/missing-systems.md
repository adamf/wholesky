# The systems that are not here yet

wholesky runs the passenger side of a day of aviation: reservations at the
carriers, distribution at the GDSes, departure control and sortation at
the airports, the datalink and ATS networks beside them, seat inventory,
schedule changes and reaccommodation. That is the spine an airline's IT
hangs off, and it is real enough now that its absences are visible. This
is the list of what a full airline runs that the world does not model,
what each would add, and what it would exercise in jetway. It is ordered
by how much of the day's traffic each one is and how much of it jetway
could carry today.

## Big, and on the wire

**Fares and pricing (ATPCO, fare rules, taxes).** Every record is sold at
no price. Real distribution shops fares before it sells: a fare basis per
segment, rules that decide whether the class is bookable for this trip,
taxes and surcharges, and a ticket amount. The missing piece is a fare
filing (ATPCO's is licensed; a synthetic fare structure per market is
easy) and a pricing step in the GDS before the sell. It would put fare
basis codes on segments, amounts on tickets, and make IROPS reaccommodate
against fare rules rather than only against seats.

**Ticketing and settlement (BSP/ARC, interline billing).** Tickets exist
as numbers and coupons, and every record now carries what it was sold for
and can be exported in full (`jetwayctl export`, v0.1.60) -- the input a
settlement file is built from; nothing is settled. A ticket sold by a GDS through
an agency is reported to the BSP, paid to the carrier, and prorated
between carriers on interline itineraries. The recorded day's 7,000
codeshares are billed between the marketing and operating carriers. This
is a batch system of files, not messages, and it is where the money is.

**Revenue management.** Done as far as the method goes (jetway v0.1.67):
`pkg/inventory` sells under nested class authorisations, and an EMSR-b
controller (Belobaba's heuristic, the textbook one) sets them from a
demand forecast by class, re-optimised on every question. Every carrier
in the world now runs it, with the tariff's forecast: the class mix the
demand draws from, total demand a tenth above the cabin, so the deep
discounts close while full fare sells to the seat. What is still missing
is the forecaster: the forecast is static, where a real system reads
history and the booking curve and moves the ladder through the days
before departure; and the network kind of RM (bid prices over an O&D)
rather than leg-based.

**Interline through check-in (IATCI).** Done (jetway v0.1.61-63, `pkg/iatci`):
one connection in four crosses carriers, the first carrier's departure
control asks the second's over the switch (DCQCKI), the second seats the
passenger on its own flight and answers (DCRCKA), and the seat rides on the
connection. The structures are the PADIS release 01.1 layouts as publicly
mirrored; the members-only implementation guide was not consulted.

**Schedules distribution (SSIM, ASM/SSM in full).** Partly. An aircraft
going technical is now a story (jetway v0.1.65): one flight in a hundred
and fifty is substituted after check-in opens, departure control rebuilds
the cabin and re-seats or denies boarding, the inventory shrinks to the new
aircraft, and distribution hears the ASM EQT and queues the bookings. Time
changes are done too (jetway v0.1.66): a flight running forty-six minutes
or more late is announced about two hours out as an ASM TIM, distribution
moves the sold segments to the new times at TK and queues the advice, and
the panel shows STD → ETD. The SSIM file is now a source too (jetway v0.1.68,
`pkg/ssim` chapter 7 records to the layout two open-source parsers share):
`worldc -ssim` writes the compiled day as one carrier file after another,
with codeshares as DEI 050/010 segment data, and `skyd -ssim` takes the
day's flights from such a file in place of the manifest's, so a carrier's
own schedule file can fly. Still the compiler's: rotations, delays and
the recorded day, which no schedule file carries.

**Baggage reconciliation and tracing (BRS, WorldTracer).** Reconciliation
is done (jetway v0.1.59): at the door every bag the hold reported loaded is
matched to a boarded passenger, unaccompanied bags hold the door until
sortation is told to pull them, short-shipped bags are named for the rush,
and the panel shows the count. Tracing is not: WorldTracer is the
interline system for the bags that got lost, and nothing here loses one
yet.

**Border and government (APIS, PNRGOV, Secure Flight, timatic).** APIS is
done (jetway v0.1.64, `pkg/paxlst`, specified against the public
WCO/IATA/ICAO guide and tested on its worked examples): every international
door close sends the border control agency the flight-close passenger list,
and the agency counts travellers by arrival. PNRGOV is done (jetway v0.1.66,
`pkg/padis`, specified against the free IATA PADIS PNRGOV guide and tested
on its worked example): every international flight pushes its records to
the state once when the name list goes out and again at the door with each
traveller's seat, sequence and bags. Secure Flight-style vetting before a
boarding pass and timatic are not.

## Operations, mostly off the wire

**Crew.** No one is flying the aircraft. Crew scheduling, tracking, legality
(duty limits), and the disruption knock-on when a crew times out are the
largest single cause of a cancellation after weather. A crew model would
make the recorded day's delays propagate for the right reasons.

**Maintenance and aircraft rotation.** Tails are rotated through the
schedule; nothing goes technical. An AOG (aircraft on ground) is a
substitution -- another tail, often another type, hence an equipment
change ASM and a re-seating of the cabin -- and that is a story every
part of the system participates in.

**Flight planning and dispatch.** The ops desk files a synthetic route.
Dispatch computes the route, fuel and alternates from weather and
performance, produces the operational flight plan the crew signs, and
the loadsheet's fuel figure comes from it. The link to ATC exists
(`pkg/ats` over `pkg/aftn`); the planning does not.

**Weather and air traffic flow.** Delays are drawn from the record. A
weather system that closes a region, ground stops and ground delay
programs (the FAA's CDM messages), and the slot swapping airlines do under
them, would make the delay model causal and give IROPS the multi-flight
cascades it exists for.

**Airport systems.** Common-use check-in (CUPPS), gate management, FIDS,
stand allocation, de-icing. The airport is a set of addresses today. Gate
and stand allocation matter for the connections story (a bag makes the
connection or does not depending on stand distance), and CUPPS is how a
DCS is actually driven at a shared airport.

**Ground handling and catering.** Ground handlers receive the load
messages the DCS already sends; the handlers themselves, turnaround
times, and the catering that follows the passenger counts are not
modelled.

**Cargo.** A passenger aircraft's belly is cargo revenue. Air waybills,
Cargo-IMP and Cargo-XML messages, ULD control, and the customs manifest
are a second messaging network the size of the passenger one. jetway
would parse Cargo-IMP the way it parses Type B; nothing has been started.

## Distribution, beyond the GDS

**NDC and ONE Order.** A slice of demand arrives as NDC orders today. A
full NDC seller with offers and orders, an order management system
replacing the PNR and the ticket, and the reconciliation between the two
worlds during the long transition, is where distribution is going.

**Direct channels and loyalty.** The carrier's own website and app, its
loyalty programme (accrual, redemption, tier), and the customer identity
that ties bookings across records. Frequent flyer numbers ride the filled
records; nothing reads them.

**Payments and fraud.** Nothing is paid for. Card authorisation at booking,
fraud screening, refunds and chargebacks are the systems that decide
whether a booking is real.

**Disruption communication.** Passengers are rebooked and nobody is told.
Notifications, rebooking self-service, vouchers and compensation
(EU261) are the passenger-facing half of IROPS.

## The network itself

**Type B over IP as the carriers actually receive it.** SITA and ARINC
deliver Type B over MQ, over HTTPS (Type B over IP with acknowledgements),
over IBM MQ queues, and as email gateways, as well as the MATIP the world
already speaks. A production jetway needs the MQ and HTTPS deliveries.

**Two switches.** Done (jetway v0.1.69, wholesky `-switches 2`): jetway's
node gained the `link_dial` egress, a dialled, bidirectional link one
switch holds open to another, and `via` routing sends the other switch's
subscribers down it; the world runs two switches joined by that trunk,
every carrier homed on one by hash, the distribution systems, networks
and border agencies on the first. A booking on a carrier across the
trunk sells and settles; a message for one local and one remote
subscriber reaches each once and is never carried back to its origin.
Still one provider's view: carriers hold a circuit to one switch, not
both, and the provider routing tables are the configuration, not a
message.

## In order

If the next months were to be spent on this list: fares and pricing
first, because everything downstream (ticketing, settlement, RM, IROPS
under rules) wants a price on the record; then revenue management, because
it makes inventory behave; then IATCI and APIS, because they are the two
message dialogues a real DCS cannot ship without; then weather and crew,
because they are why days go wrong. Cargo is a second project with the
same shape as the first one.
