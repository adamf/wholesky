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

**Revenue management.** The inventory sells while a cabin has a seat. A
real carrier sells by booking class with authorisation levels that yield
management moves every night against demand forecasts, and closes cheap
classes on flights that are filling. The inventory has the hooks
(`pkg/inventory` pools by cabin; classes map onto cabins) but no nesting
and no controller. Adding class authorisations and a nightly optimiser
would make availability broadcasts change through the day the way they
do, and give the filler a reason for the fare mix it invents.

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
changes (TIM) and the SSIM file as the schedule's source are not yet
applied; the world still compiles its own timetable.

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
and the agency counts travellers by arrival. Secure Flight-style vetting
before a boarding pass, PNRGOV pushes and timatic are not.

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

**Two switches.** The world has one switch. The real network has two
providers and carriers hold circuits to both; the switches exchange
traffic; addresses are routed by provider tables. A second switch and the
inter-switch trunk is the piece full-throttle.md identified as the
scaling step, and it is also the realism step.

## In order

If the next months were to be spent on this list: fares and pricing
first, because everything downstream (ticketing, settlement, RM, IROPS
under rules) wants a price on the record; then revenue management, because
it makes inventory behave; then IATCI and APIS, because they are the two
message dialogues a real DCS cannot ship without; then weather and crew,
because they are why days go wrong. Cargo is a second project with the
same shape as the first one.
