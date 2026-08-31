# wholesky

One Earth-day of passenger aviation, simulated on
[Jetway](https://github.com/adamf/jetway): every scheduled flight, every
booking, and every Type B message that makes it go, over real sockets.

Design: "The Whole Sky" (design note, 2026-08-31). The short version — the
sky is loud but it is not big. The world's reservation fabric peaks at a few
thousand messages a second; simulated honestly, with availability at its real
churn and a baggage message per bag per scan, it is still only tens of
thousands. What is interesting is not throughput but topology, and the
topology here mirrors the real one:

- **The switch** is a Jetway node in relay mode: the SITA role. Everyone
  holds one link to it and addresses everyone else by teletype address.
- **Carriers** are tenants of a host process — most of the world's carriers
  are tenants of a handful of hosted reservation systems, so the efficient
  shape and the realistic shape are the same shape.
- **The GDS** is a Jetway gateway with one switch link and a peer entry per
  carrier. A booking's sell crosses two links and comes back confirmed.
- **OTAs are not nodes.** They are demand: load generators speaking NDC and
  booking APIs. (Phase two.)

## Running it

```sh
go run ./cmd/worldc -countries "United Kingdom,France,Germany" -carriers 30 -o europe.json
go run ./cmd/skyd -world europe.json -carriers 12 -book 8 -warp 240
```

`worldc` compiles a deterministic manifest — airports, carriers, daily
flights — from the vendored OpenFlights snapshot. Same seed, same world.
`skyd` boots it: switch, tenants, GDS, real TCP on loopback, then pushes
bookings through the fabric and runs the flight day at `-warp`, emitting a
real MVT for every departure and arrival. The switch's console is Jetway's
own, on `-console`, where every message can be opened and read field by field.

## Status

Phase one, early. Works: world compilation at any scale from a continent to
the planet; the switch/tenant/GDS topology over real sockets; free-sale and
request-reply bookings through the switch; the flight day as MVT traffic.
Not yet: EDIFACT relay at the switch (phase-one worlds force Type B), demand
generation beyond smoke bookings, PNL/ADL and baggage, the globe.

## Data

`data/` vendors the [OpenFlights](https://openflights.org) database
(airports, airlines, routes), Open Database License. The snapshot is
deliberately allowed to be stale: the simulation needs a plausible planet,
not this year's planet. A current OAG or Cirium snapshot dropped through the
same compiler produces today's world instead.
