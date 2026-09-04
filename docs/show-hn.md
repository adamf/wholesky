# Show HN draft

**Title (78 chars):**

Show HN: Wholesky – a simulation of the world's airline network, on real wire protocols

**Alternative titles:**

- Show HN: 518 airline reservation systems, 3 GDSes and 2 switches talking Type B over real TCP, live
- Show HN: I simulated the day before Thanksgiving, every US flight, then priced, flew and settled it

**Text:**

Wholesky flies a day of passenger aviation the way the industry's systems
do it: every carrier is its own reservations and departure-control system,
three GDSes sell into them, two message switches hold a trunk between them,
and everything that moves between any of them is a real Type B or EDIFACT
message over a real socket. The globe draws only what the messages say.

Live: https://wholesky.io → https://wholesky-demo.fly.dev/eye (a synthetic
day: 518 carriers, 103,688 flights, real routes, invented timetables,
8,500 aircraft aloft at the evening peak)

The recorded day: https://wholesky-thanksgiving.fly.dev/eye (the Wednesday
before Thanksgiving 2025 from BTS on-time data: 22,889 US flights, real
tails, real delays by cause, 64 cancellations, 44 diversions, at real time,
filled to a holiday load of 1.4 million priced bookings)

Code: https://github.com/adamf/wholesky and the engine underneath it,
https://github.com/adamf/jetway (Go, MIT, an airline messaging gateway that
is also a GDS, seat inventory, fare engine, DCS, settlement writer and
switch, meant to be usable for real, not only for this)

Click any aircraft and you get what its carrier holds: the name list that
went to the airport at T-180 (a full 737 is three parts), the records
pushed to the state's passenger information unit (PNRGOV), check-in in
waves with seat assignments and bag tags to sortation, through check-in
across carriers over EDIFACT, the ADL at T-60, boarding, the bag report
from the hold, the door closing at T-10 and the final sales, transfer,
service and load messages and an AHM 560-method loadsheet; the bag left
behind, rushed on the next flight and traced at the other end; the
aircraft's OUT/OFF/ON/IN over a datalink provider, from which the MVT is
derived; the flight plan filed with the towers over the AFTN and their DEP
and ARR back; the manifest name by name; the bookings behind the names
under the locators the selling channels issued, each with its fare and
fare basis; and, on a cancelled flight, where each passenger went.

Then the money. Every booking is priced from a synthetic tariff as of its
purchase date. Each cabin sells under nested authorisations set by EMSR-b
from a forecaster reading the booking curve, with bid prices from a network
LP over each connecting point. At the end of the day a Billing and
Settlement Plan runs: the carrier gets its HOT file to IATA's public DISH
23 handbook, column by column, with refunds, exchanges and agency memos;
the agents' RET is written; the carrier that flew a codeshare coupon
invoices the carrier that sold it, prorated by mileage. All of it is
served as files you can open.

Things I learned building it that surprised me:

- The sky is loud but not big. Real global reservations traffic is a few
  thousand messages a second. The switch peaks above 16,000/s on one
  4-vCPU machine during the departure banks. Topology is the interesting
  part, not throughput: the second switch and the trunk found two
  deadlocks the single switch never could.
- Real load finds real bugs. Filling the recorded day to a holiday load
  broke a Type B line-length rule with the first family of four, overran
  the 60-line message envelope, and deadlocked a link because both ends
  answered from inside their read loops. The first run of the release
  gate found 88 oversold cabins, 83 of them business cabins on Hawaii legs.
  All fixed upstream, each with a test watched to fail first. Eighty-four
  jetway releases so far.
- Flight numbers are not flight identifiers. Southwest flies one number
  over several legs a day. Everything keyed on number alone was wrong.
- A seat inventory has to be rebuilt from the book of record at boot.
  Anything that remembers sold seats in memory oversells after a restart.
- Free specs exist for more than you would think. IATA's PNRGOV guide,
  the DISH 23 settlement handbook, the PAXLST guide, ICAO Annex 10 and the
  Type B whitepaper are all public, and each one corrected bugs whose
  tests had encoded the same guess as the code. What is paywalled
  (AIRIMP, SSIM, WorldTracer's formats) is implemented as a stated
  profile, and a table in the repo says which is which.

What is synthetic and labelled as such: the passengers (a generator fills
the schedule deterministically from a seed), aircraft types inferred from
carrier and stage length, seat counts, the tariff (ATPCO's filings are
licensed; this one is shaped from distances, so the money is a shape, not
a number), and the settlement that follows from it. What is real: the
schedule on the recorded day, the wire formats and their published rules,
the state separation (nothing moves between systems except a message), and
the failure modes.

I would especially like to hear from anyone who has run a reservations
system, a Type B network, a DCS, a revenue management desk or a BSP
reconciliation. It looks wrong in ways I cannot name yet and you can.
