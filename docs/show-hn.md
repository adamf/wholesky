# Show HN draft

**Title (78 chars):**

Show HN: Wholesky – a simulation of the world's airline network, on real wire protocols

**Alternative titles:**

- Show HN: 518 airline reservation systems, 3 GDSes and 2 switches talking Type B over real TCP, live
- Show HN: I simulated the day before Thanksgiving, every US flight, then priced, flew and settled it

**Text:**

Wholesky simulates a day of passenger aviation with the systems the
industry uses. Every carrier has its own reservations system and
departure control system (DCS). The world has 3 global distribution
systems (GDSes), which sell into the carriers. It has 2 message switches
with a trunk between them. Every message between these systems is a
Type B or EDIFACT message on a TCP socket. The globe shows only what the
messages contain.

Live: https://wholesky.io → https://wholesky-demo.fly.dev/eye. This is a
synthetic day. It has 518 carriers, 103,688 flights, routes as flown,
invented timetables and 8,500 aircraft aloft at the evening peak.

The recorded day: https://wholesky-thanksgiving.fly.dev/eye. This is the
Wednesday before Thanksgiving 2025, built from Bureau of Transportation
Statistics (BTS) on-time data. It has 22,889 US flights, the tails that
flew them, their delays by cause, 64 cancellations and 44 diversions. The
simulated day runs at the speed of the wall clock. It is filled to a
holiday load of 1.4 million priced bookings.

Code: https://github.com/adamf/wholesky. The engine underneath it is
https://github.com/adamf/jetway. Jetway is written in Go under the MIT
licence. It is an airline messaging gateway. It is also a GDS, a seat
inventory, a fare engine, a DCS, a settlement writer and a switch. It is
built for production use as well as for this simulation.

You can also run one of the airlines. Every carrier runs on autopilot.
Take a seat at https://wholesky-demo.fly.dev/ops/ and choose which
departments to run yourself. For each event in those departments, the
simulator sends you a decision with a default and a deadline. Examples
are a slot to take or ask to improve, and a delay to announce or hold.
Others are a rival's fare cut to match or hold, and bags to rush or hold.
Agents get the same API as Model Context Protocol (MCP) tools. Every run is recorded.
The replay shows the simulated clock on every line. Claude ran Jet2 for a
day: 159 decisions, none defaulted, no cancellations, 87 notes on the
reasons. The replay is at https://wholesky.io/replay/?src=jet2-claude.json.
The terminal Claude used is at https://wholesky.io/replay/terminal.html

Click any aircraft to see what its carrier holds for the flight. The name
list goes to the airport at T-180. The list for a full 737 has 3 parts.
The carrier pushes the records to the state's passenger information unit
as PNRGOV messages. Check-in runs in waves, with seat assignments and bag
tags sent to sortation. Through check-in across carriers runs over
EDIFACT. The additions and deletions list (ADL) goes out at T-60. Boarding
follows. The bag report comes from the hold. The door closes at T-10. The
final sales, transfer, service and load messages follow, with an AHM
560-method loadsheet. A bag left behind is rushed on the next flight and
traced at the other end. The aircraft sends OUT/OFF/ON/IN over a datalink
provider, and the carrier derives the MVT from those times. The carrier
files the flight plan with the towers over the Aeronautical Fixed
Telecommunication Network (AFTN). The towers send DEP and ARR back. The
manifest lists every name. The bookings behind the names are shown under
the locators the selling channels issued, each with its fare and fare
basis. On a cancelled flight, the page shows where each passenger went.

The world also simulates the money. Every booking carries a price from a
synthetic tariff, as of its purchase date. Each cabin sells under nested
authorisations. An expected marginal seat revenue (EMSR-b) controller sets
the authorisations from a forecaster that reads the booking curve. A
network linear programme (LP) over each connecting point sets the bid
prices. When the sim day ends, a Billing and Settlement Plan (BSP) runs.
Each carrier gets its HOT file, with refunds, exchanges and agency memos.
The file is laid out column by column to the International Air Transport
Association (IATA) public DISH 23 handbook. The plan also writes the
agents' RET file. The carrier that flew a codeshare coupon invoices the carrier
that sold it, prorated by mileage. All of these are served as files you
can open.

Things that surprised me while building it:

- The message volume is small. The industry's global reservations
  traffic is a few thousand messages a second. The switch peaks above
  16,000 messages a second on a single 4-vCPU machine during the
  departure banks. Topology is more interesting than throughput. The
  second switch and the trunk exposed 2 deadlocks that a single switch
  could not.
- Full loads expose bugs. Filling the recorded day to a holiday load broke
  a Type B line-length rule with the first family of 4. It overran the
  60-line message envelope. It deadlocked a link because both ends
  answered from inside their read loops. The first run of the release
  gate found 88 oversold cabins. 83 of them were business cabins on Hawaii
  legs. Each fix is in jetway with a regression test. There have been 95
  jetway releases so far.
- A flight number does not identify a flight. Southwest flies 1 number
  over several legs a day. Everything keyed on the number alone was wrong.
- At boot, the system must rebuild the seat inventory from the book of
  record. An inventory that holds sold seats only in memory oversells
  after a restart.
- Letting strangers run the airlines required a security pass first. 6
  audits over both codebases found problems that a private deployment
  never meets. The control plane between the world's machines was open.
  The switch ports accepted any peer's claimed name. A page reflected its
  URL. The fixes and the open decisions are in
  https://github.com/adamf/wholesky/blob/main/docs/security.md
- Free specifications exist for more formats than I expected. IATA's
  PNRGOV guide, the DISH 23 settlement handbook and the PAXLST guide are
  public. So are International Civil Aviation Organization (ICAO) Annex 10
  and the Type B whitepaper. Each one corrected bugs whose tests had
  encoded the same guess as the code. AIRIMP, SSIM and WorldTracer's
  formats are paywalled. Those are implemented as a stated profile. A
  table in the repo says which formats follow a public specification and
  which follow a profile.

These parts are synthetic, and labelled as synthetic: the passengers, the
aircraft types, the seat counts and the tariff. The settlement that
follows from the tariff is synthetic too. A generator fills the schedule with passengers
deterministically from a seed. Wholesky infers aircraft types from carrier
and stage length. ATPCO's fare filings are licensed, so the tariff is
shaped from distances. The amounts follow the shape of a tariff. They are
not the industry's figures. These parts are not synthetic: the schedule on
the recorded day, the wire formats and their published rules, the state
separation and the failure modes. Systems communicate only by messages.

I would especially like to hear from anyone who has run a reservations
system, a Type B network or a DCS. Anyone who has run a revenue
management desk or a BSP reconciliation is equally welcome. It is
probably wrong in ways that I cannot name yet and that you can.
