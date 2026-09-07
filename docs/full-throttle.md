# Full throttle

This document estimates the machines needed to run the whole world with
every flight filled to the loads the industry carries. It gives 3
definitions, because "full throttle" means a different machine count at
each speed of the day. Numbers marked *measured* come from the running
system on 2026-09-01. The other numbers are scaled from the measured
ones, and the text says so.

## The deployed world today

| | measured |
| --- | --- |
| Flights per day | 103,688 across 518 carriers |
| Demand | 3,510 bookings / real minute across three GDS machines (real global reservations volume is about 5M a day, which is this rate for 24 hours) |
| Fabric peak, warp 60 | 16,315 msg/s window-max through the switch, of which res 11,246, AVS 1,331, DCS 760, MVT 448, PNL 239 |
| Airborne at peak | ~17,500 aircraft |
| Live heap per stored record (record + events) | 2.7 KB; 4.8 KB with GC slack |
| Off-peak load, warp 6 | core 0.39 / 2 vCPU, gds 0.34 / 1 vCPU, region 0.31 / 2 vCPU |
| Memory in use | core 0.5 GB, gds 1.9 GB (the world-sized availability cache), region 0.9 GB, of 4 GB each |
| Tenant book of record | one Managed Postgres (launch plan, 10 GB), purged at each day wrap |

One fact decides everything below. **Time is compressed and demand is
not.** At warp 60 a day takes 24 minutes and receives 84k bookings. At
warp 6 a day takes 4 hours and receives 840k bookings. At warp 1 a day
takes 24 hours and receives 5M bookings, which is the industry's number.
In the industry a flight carries about 48 bookings, or about 100
passengers. Today, at warp 6 and selling only the flown day, a flight gets
about 8 bookings.

## Definition A, warp 1 at the industry's loads

The day takes 24 hours. Demand stays at its present rate. Every flight
fills to the industry's load, because 3,510 bookings a minute is the
industry's rate.

| component | need | today |
| --- | --- | --- |
| Switch | 16k msg/s peaks, same as now | performance-2x, idle off-peak |
| GDS ×3 | same booking rate as now | performance-1x each, 1.9 GB used |
| Regions ×2 | DCS manifests for ~26k open flights × 100 passengers ≈ 1 GB per region in memory | 4 GB, 0.9 GB used |
| Postgres | 5M records + ~25M events per day live before the purge: **30–40 GB**, ~300 row writes/s sustained | 10 GB volume. **The one change**: 100 GB and the Scale plan for the write IOPS |

Cost: the 6 machines as they are, about $400/month, plus a larger cluster
at about $100–150/month more. Work: set `-warp 1` on the core, resize the
volume, and watch the first day-wrap purge delete 5M rows. The purge is
paced per tenant. At this size it should become a partition drop, as
described below.

The loss is speed. The departure banks take hours to arrive, and the
globe shows a live map rather than a time-lapse. The gain is a day at the
industry's pace and loads. That is also the shape the BTS replay needs.

## Definition B, warp 6 at the industry's loads

Definition B runs a 4-hour day at warp 6 with demand multiplied by 6.
That is 21,000 bookings a minute. Everything scales linearly with
bookings. The flight day does not scale.

| component | need | how |
| --- | --- | --- |
| Switch | ~100k msg/s | Not one process. Either measure a performance-8x with `cmd/jetwayload` and hope, or shard: N switches trunked to each other by address range, GDSes connected to all. Jetway has address relay; it does not have **switch-to-switch trunking**, which is the piece to build |
| GDS | 6× today's booking rate | 18 GDS nodes, or 3 × performance-6x if `Book` scales with cores (measure: today one GDS does 20 bookings/s on one vCPU with headroom) |
| Regions | 6× the sell traffic into tenants, same DCS load | 4–6 regions on performance-2x |
| Postgres | 350 records/s + 1,750 event rows/s + updates; 5M records per 4 hours | Performance plan; or one cluster per region pair |
| Demand generator | 21k bookings/min of goroutines | rides on the GDS machines; scales with them |

Cost: about 25–30 performance machines at $2.5–3.5k/month, plus Postgres.
Work: switch trunking in jetway, a `jetwayload` run, and the GDS
availability cache memory. Switch trunking is a production feature,
because SITA's switching centres work this way. The `jetwayload` run
replaces the per-core guess with a measured number. The availability
cache is world-sized per GDS, at 500 MB each, so more GDSes means more
copies. String interning is the known fix.

## Definition C, warp 60 at the industry's loads

Definition C runs a 24-minute day at warp 60, with demand multiplied
by 60. That is 210,000 bookings a minute and about 1 million messages a
second through the fabric. It needs more than 20 switch cores as trunked
shards, about 40 GDS nodes and about 20 regions. Postgres is sharded
across at least 4 clusters at about 17k row writes/s in total. The cost
is about $8–12k/month, and every jetway change from B must come first.
This definition is not recommended. It buys a time-lapse. Definition A
buys a day at the industry's pace and loads for the price of a disk.

## Jetway work for each definition

- **A:** none required. An optional change is to make the tenant purge a
  partition drop, with records partitioned by day as the message log
  already is. A `DELETE` of 5M rows costs a minute of I/O while the day's
  first bank departs.
- **B:** switch-to-switch trunking, a load-test number for messages per
  core, and availability cache interning.
- **C:** all of B, plus sharding of the availability cache and the demand
  generator across many GDS nodes with consistent partner routing.

## Recommendation

Go to **A**. It is the only definition where the day runs at the
industry's pace and loads without compression. It costs a disk. It is the
shape the Thanksgiving-Eve BTS replay needs: a recorded day's schedule at
the industry's pace and loads. Keep warp 6 as the demo's default pace. At
warp 6 a person can watch a departure go through check-in and close in
40 minutes. Switch to warp 1 for the recorded-day runs.

## Filling the recorded day with bookings

Definition A settles the pace. It does not settle the passengers. The
recorded day flies 22,889 flights from the record. The demand generator
books them as it books the synthetic world. It sends a stream of sells to
the distribution systems, 800 a minute, spread over every flight of the
day. Over a 24-hour day that is 1.15 million bookings. The bookings arrive as
the day runs. A flight that closes at 06:00 has had 6 hours of selling. A
flight that closes at 23:00 has had 23 hours. Both are far short of the
load the Wednesday before Thanksgiving carried. The name lists are short,
the check-in counters are quiet and the loadsheets are light.

The fix is the one the industry has: the day's bookings exist before the
day starts. `internal/fill`, built on 2 Sep 2026 and run with
`skyd -fill`, reads the compiled manifest. It writes each carrier's book
of record for the day into the tenants' Postgres before the flight day
runs. The book holds every passenger name record (PNR) that would hold a
seat at 00:00. The demand generator then adds what arrives on the day of
travel: the late trickle of same-day sells, changes and cancellations.

The filler generates these items per flight from a load factor and a seed:

- **Parties**: 1 to 6 names, weighted for holiday travel, with more
  families and fewer single travellers than a Tuesday in February. The
  names are plausible names with titles. A share of the passengers are
  children and infants.
- **Itineraries** consistent with the schedule. A quarter of the
  passengers connect through the operating carrier's hubs on legs that
  connect in the schedule. The filler respects the minimum connect time,
  and the inbound lands before the outbound closes. This gives the PTM and
  the misconnect handling passengers to work with. A share of the
  passengers travel on the codeshares the record names, for example sold
  as DL3991 and flown by OO.
- **Booking classes** by fare bucket, and **SSRs** at a plausible rate
  (WCHR, CHLD, INFT, meal codes). **Tickets** are issued against each
  name. A **frequent flyer** element is on the share of passengers that
  would have one.
- **Locators**: the carrier's own record locator and the selling channel's
  locator, in the record's `Locators`. The selling channel is 1G, 1S, 1A
  or the carrier's direct channel. The drill-through then shows the
  passenger's locator from the selling channel, and the console shows the
  record under both locators.
- **Booking dates** spread over the months before the day. The record
  histories then look like the industry's records. A slice of the records
  are already cancelled or changed.

The fill has a storage cost. A 2025 Thanksgiving-eve load factor of about
85% over the day's 3.4 million seats is 2.9 million passengers. At 1.6
names a party, that is 1.8 million PNRs. The tenants' records take about
2.7 KB live and somewhat less as `jsonb`. The day's books are therefore
4-5 GB in Postgres with indexes. That fits the 10 GB plan only if the
distribution systems' copies are not kept too, or if the plan grows.
Writing the records through `COPY` at 20,000 rows a second takes a minute
and a half. Writing them through the store API takes 10 times as long.
The generator is deterministic from its seed, so there is no dump to
keep. The end-of-day purge runs, and the filler runs again for the next
day. The filler runs either as the first step of the flight day or as a
`skyd -fill` pass before it.

The fill changes the rest of the day. The name lists become full name
lists. A 737 at 85% has 150 names and 3 PNL parts. The counters check in
2.9 million passengers a day, with 1.7 million bags tagged and messaged
to sortation. The closures carry holiday loads. The arrival stations
receive transfer lists with passengers on them. The IROPS engine has
whole planeloads to reaccommodate when the record says a flight was
cancelled. Departure control holds a flight in memory from T-180 to
arrival. The window is therefore 3 to 6 hours of the day's flights at
once, about 400,000 passengers and some 250 MB. The performance-2x
machine already has that memory.

Measured: Southwest's day at 0.85 is 4,222 flights, 260,037 records and
528,956 passengers. The rows take 2.2 KB each on disk with indexes and
24 seconds to write on a laptop. The whole day scales to about 1.4
million records and 3.5 GB. The recorded-day app runs at 0.6 until a test
shows that the purge's dead tuples and the refill fit inside the 10 GB
plan together.

Building the fill decided the following. For now, the distribution
systems do not hold the same records. Today their ledgers are in memory
and bounded. On the recorded day the correct shape is a Postgres node for
each distribution system. A booking then exists at the channel that sold
it as well as at the carrier that flies it. A change made at either
crosses the wire to the other. That doubles the storage. The alternative
is to treat the pre-day bookings as already purged from the GDSes' active
files. The carriers alone then hold the bookings, with the selling
channel's locator kept on the record. A GDS purges a PNR from its active files
after departure, not before. The first option is correct. The second
option is cheaper and puts the passengers on the aircraft, which is the
goal.

## Later changes

The numbers above moved after 3 changes. The regions ran out of memory
every few hours on a filled day. The carriers' books kept every record
and its history until the day wrapped, and the settlement kept every
airline's HOT file. Flown records now leave 3 hours after their last
departure, through jetway's `store.Pruner`. Availability beliefs leave
with their flights. The settlement builds a HOT file on request. The
lobby answered in 20 seconds because it asked every machine and every
joined world on each request. The lobby is now a cache. The core rebuilds
the cache in the background and asks all peers concurrently. The lobby
answers in under a second. A carrier's state page took minutes on a
filled book. jetway's inventory answered "seats sold by class" with a
scan of every key it held. Since v0.1.95 the inventory is indexed
by pool. The biggest carrier's state page answers in half a second.
