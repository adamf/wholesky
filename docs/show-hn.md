# Show HN draft

**Title (78 chars):**

Show HN: Wholesky – a simulation of the world's airline network, on real wire protocols

**Alternative titles:**

- Show HN: I simulated the day before Thanksgiving, every US flight, on the airlines' own message protocols
- Show HN: 518 airline reservation systems and 3 GDSes talking Type B over real TCP, live

**Text:**

Wholesky flies a day of passenger aviation the way the industry's systems
do it: every carrier is its own reservations and departure-control system,
three GDSes sell into them, and everything between them is a real Type B or
EDIFACT message over a real socket. The globe draws only what the messages
say.

Live: https://wholesky-demo.fly.dev/eye (a synthetic day: 518 carriers,
103,688 flights, real routes, invented timetables)

The recorded day: https://wholesky-thanksgiving.fly.dev/eye (the Wednesday
before Thanksgiving 2025 from BTS on-time data: 22,889 US flights, real
tails, real delays by cause, 64 cancellations, 44 diversions, at real time)

Code: https://github.com/adamf/wholesky and the engine underneath it,
https://github.com/adamf/jetway (Go, MIT, an airline messaging gateway,
GDS, DCS and switch that is meant to be usable for real, not only for this)

Click any aircraft and you get what its carrier holds: the name list that
went to the airport at T-180 (a full 737 is three parts), check-in in
waves with seat assignments and bag tags to sortation, the ADL at T-60,
boarding, the bag report from the hold, the door closing at T-10 and the
final sales, transfer, service and load messages and an AHM 560-method
loadsheet; the aircraft's OUT/OFF/ON/IN over a datalink provider, from
which the MVT is derived; the flight plan filed with the towers over the
AFTN and their DEP and ARR back; the manifest name by name; the bookings
behind the names under the locators the selling channels issued; and, on
a cancelled flight, where each passenger went.

Things I learned building it that surprised me:

- The sky is loud but not big. Real global reservations traffic is a few
  thousand messages a second. The switch peaks above 16,000/s on one
  4-vCPU machine during the departure banks. Topology is the interesting
  part, not throughput.
- Real load finds real bugs. Filling the recorded day to a holiday load
  (about a million pre-sold bookings, written straight into Postgres)
  broke a Type B line-length rule with the first family of four, overran
  the 60-line message envelope, and deadlocked a link because both ends
  answered from inside their read loops. All fixed upstream with tests
  that were watched to fail first.
- Flight numbers are not flight identifiers. Southwest flies one number
  over several legs a day. Everything keyed on number alone was wrong.
- A seat inventory has to be rebuilt from the book of record at boot.
  Anything that remembers sold seats in memory oversells after a restart.

What is synthetic and labelled as such: the passengers (a generator fills
the schedule deterministically from a seed), aircraft types inferred from
carrier and stage length, seat counts, and fares (there are none: nothing
is priced or paid for). What is real: the schedule on the recorded day,
the wire formats and their published rules, the state separation (nothing
moves between systems except a message), and the failure modes.

I would especially like to hear from anyone who has run a reservations
system, a Type B network, a DCS or an ops centre. It looks wrong in ways I
cannot name yet and you can.
