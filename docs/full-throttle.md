# Full throttle

What it takes to fly the whole sky with every seat sold the way the real
sky is. Three definitions, because "full throttle" means a different
machine count depending on how fast the day runs. Numbers marked
*measured* came off the running system on 2026-09-01; the rest are scaled
from them and say so.

## Where the deployed world stands

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

The thing that decides everything below: **time is compressed and demand is
not.** At warp 60 a day is 24 minutes and receives 84k bookings; at warp 6
it is 4 hours and receives 840k; at warp 1 it is a day and receives 5M,
which is the real number. Real flights carry about 48 bookings (~100
passengers) each. Today, at warp 6 selling only the flown day, a flight
gets about 8.

## Definition A — real time, real loads (warp 1)

The day takes a day. Demand stays exactly where it is and every flight
fills to a real load, because 3,510 bookings a minute *is* the real rate.

| component | need | today |
| --- | --- | --- |
| Switch | 16k msg/s peaks, same as now | performance-2x, idle off-peak |
| GDS ×3 | same booking rate as now | performance-1x each, 1.9 GB used |
| Regions ×2 | DCS manifests for ~26k open flights × 100 passengers ≈ 1 GB per region in memory | 4 GB, 0.9 GB used |
| Postgres | 5M records + ~25M events per day live before the purge: **30–40 GB**, ~300 row writes/s sustained | 10 GB volume — **the one change**: 100 GB and the Scale plan for the write IOPS |

Cost: the six machines as they are (~$400/month) plus a larger cluster
(order $100–150/month more). Work: `-warp 1` on core, a volume resize, and
watching the first day-wrap purge delete 5M rows (it is paced per tenant;
at this size it should become a partition drop — see below).

What you lose: the departure banks take real hours to arrive, so the
globe is a live map rather than a time-lapse. What you gain: a genuinely
real day, and the shape a real BTS replay wants anyway.

## Definition B — a four-hour day at real loads (warp 6, demand ×6)

21,000 bookings a minute. Everything scales linearly with bookings except
the flight day, which does not scale at all.

| component | need | how |
| --- | --- | --- |
| Switch | ~100k msg/s | Not one process. Either measure a performance-8x with `cmd/jetwayload` and hope, or shard: N switches trunked to each other by address range, GDSes connected to all. Jetway has address relay; it does not have **switch-to-switch trunking**, which is the piece to build |
| GDS | 6× today's booking rate | 18 GDS nodes, or 3 × performance-6x if `Book` scales with cores (measure: today one GDS does 20 bookings/s on one vCPU with headroom) |
| Regions | 6× the sell traffic into tenants, same DCS load | 4–6 regions on performance-2x |
| Postgres | 350 records/s + 1,750 event rows/s + updates; 5M records per 4 hours | Performance plan; or one cluster per region pair |
| Demand generator | 21k bookings/min of goroutines | rides on the GDS machines; scales with them |

Cost: roughly 25–30 performance machines, $2.5–3.5k/month, plus Postgres.
Work: switch trunking in jetway (real feature: this is what SITA's switching
centres do), a `jetwayload` run to replace the per-core guess with a number,
and GDS availability cache memory (the cache is world-sized per GDS — 500 MB
each — so more GDSes is more copies; string interning is the known fix).

## Definition C — a 24-minute day at real loads (warp 60, demand ×60)

210,000 bookings a minute; about a million messages a second through the
fabric. 20+ switch cores as trunked shards, ~40 GDS nodes, ~20 regions,
Postgres sharded across at least four clusters at ~17k row writes/s total.
Order $8–12k/month, and every jetway change from B first. Not recommended:
it buys a time-lapse, and A buys the truth for the price of a disk.

## Jetway work each definition implies

- **A:** none required. Optional: make the tenant purge a partition drop
  (records partitioned by day, like the message log already is), because
  `DELETE` of 5M rows is a minute of I/O the day's first bank does not want.
- **B:** switch-to-switch trunking; a load-test number for messages per
  core; availability cache interning.
- **C:** all of B, plus sharding the availability cache and the demand
  generator across many GDS nodes with consistent partner routing.

## Recommendation

Go to **A**. It is the only definition where "full throttle" means the real
thing rather than a compressed imitation of it, it costs a disk, and it is
the shape the Thanksgiving-Eve BTS replay needs: a real day's schedule, at
real time, with real loads. Keep warp 6 as the demo's default pace (a
person can watch a departure go through check-in and close in forty
minutes), and switch to warp 1 for the recorded-day runs.

## Filling the day: the bookings a recorded schedule needs

Definition A settles the pace. It does not settle the passengers. The
recorded day flies 22,889 real flights, and the demand generator books them
the way it books the synthetic world: a stream of sells at the distribution
systems, 800 a minute, spread over every flight of the day. Over a 24-hour
day that is 1.15 million bookings -- but they arrive as the day runs, so a
flight closing at 06:00 has had six hours of selling and one closing at
23:00 has had twenty-three, and both are a long way short of the load the
Wednesday before Thanksgiving actually carried. The name lists are short,
the counters are quiet, the loadsheets are light. The wire is real; the
aeroplane is empty.

The realistic fix is the one the industry has: the day's bookings exist
before the day starts. `internal/fill` (built 2 Sep 2026, `skyd -fill`)
reads the compiled manifest and writes
each carrier's book of record for the day -- every PNR that would be
holding a seat at 00:00 -- straight into the tenants' Postgres, before the
flight day runs. The demand generator then rides on top as what it really
is on the day of travel: the late trickle of same-day sells, changes and
cancellations.

What it generates, per flight, from a load factor and a seed:

- **Parties**: 1 to 6 names, weighted the way holiday travel is (more
  families, fewer singles than a Tuesday in February), with real-shaped
  names, titles, and a share of children and infants.
- **Itineraries** that are consistent with the schedule: a quarter of
  passengers connecting through the operating carrier's hubs on legs that
  actually connect (minimum connect time respected, the inbound landing
  before the outbound closes), so the PTM and the misconnect story have
  something to work with; a share on the codeshares the record names
  (sold as DL3991, flown by OO).
- **Booking classes** by fare bucket, **SSRs** at a real rate (WCHR, CHLD,
  INFT, meal codes), **tickets** issued against each name, and a **frequent
  flyer** element on the share that would have one.
- **Locators**: the carrier's own record locator, and the selling channel's
  -- 1G, 1S, 1A or the carrier's direct channel -- in the record's
  `Locators`, so the drill-through shows the passenger the locator they
  were given and the console shows the record under both.
- **Booking dates** spread over the months before, so the record histories
  read as records do, and a slice of them already cancelled or changed.

What it costs. A 2025 Thanksgiving-eve load factor of about 85% over the
day's 3.4 million seats is 2.9 million passengers; at 1.6 names a party
that is 1.8 million PNRs. The tenants' records run about 2.7KB live and
somewhat less as `jsonb`, so the day's books are 4-5GB in Postgres with
indexes -- inside the 10GB plan only if the distribution systems' copies
are not kept too, or the plan grows. Writing them through `COPY` at 20,000
rows a second is a minute and a half; through the store API, ten times
that. The generator is
deterministic from its seed, so there is no dump to keep: the end-of-day
purge runs, and the filler runs again for the next day, either as the
first step of the flight day or as a `skyd -fill` pass before it.

What it changes downstream. The name lists become real name lists (a 737
at 85% is 150 names, three PNL parts), the counters check in 2.9 million
passengers a day with 1.7 million bags tagged and messaged to sortation,
the closures carry real loads, the arrival stations receive transfer lists
that mean something, and the IROPS engine has whole planeloads to
reaccommodate when the record says a flight was cancelled. Departure
control holds a flight in memory from T-180 to arrival, so the window is
three to six hours of the day's flights at once -- 400,000 passengers,
some 250MB -- which the performance-2x machine already has.

Measured: Southwest's day at 0.85 is 4,222 flights, 260,037 records,
528,956 passengers, 2.2KB a row on disk with indexes and 24 seconds to
write on a laptop; the whole day scales to about 1.4 million records and
3.5GB. The recorded-day app runs at 0.6 until the purge's dead tuples and
the refill have been watched to fit inside the 10GB plan together.

The one thing decided by building it: the distribution systems do not hold
the same records, for now. Today their ledgers are in memory and
bounded; on the recorded day the honest shape is a Postgres node each, so
a booking exists at the channel that sold it as well as at the carrier
that flies it, and a change made at either crosses the wire to the other.
That doubles the storage. The alternative is to treat the pre-day
bookings as already purged from the GDSes' active files (which is what
happens to a GDS PNR after departure, not before) and hold them only at
the carriers, with the selling channel's locator kept on the record. The
first is right; the second is cheaper and gets the passengers on the
aeroplanes, which is the point.
