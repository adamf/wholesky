# Run a carrier

wholesky flies 518 carriers on autopilot. This is how a person, or an agent,
takes one over -- and where that leads: many people running many carriers,
their own jetway nodes on the wire, worlds joined to worlds.

Live: **https://wholesky-demo.fly.dev/ops/** (the lobby), then
`/ops/<code>` for a carrier's operations centre. The same thing as MCP tools:
`go run ./cmd/skyagent -world https://wholesky-demo.fly.dev`.

## The idea

Every carrier in the world is already an airline's systems: a reservations
host answering the distribution systems over Type B and EDIFACT, a seat
inventory under revenue management, a departure control system at every
airport it touches, an operations desk filing flight plans and reading the
towers, a bag office, and a settlement position at the end of the day. What
runs those systems today is the autopilot: rules in `internal/host` and
`internal/sim` that decide, at every point a real airline has a person
deciding, what the airline does.

A **seat** is someone taking those decisions instead. Taking a seat changes
nothing by itself; every **department** stays on autopilot until the seat
takes it manual. Where a department is manual, the simulation stops deciding
and **asks**: a decision appears in the seat's inbox with the situation, the
options, what each costs, a default and a deadline. What comes back is what
happens -- on the wire, as the real messages. An unanswered decision falls to
the default when the deadline passes, so a slow player degrades into the
autopilot and never into chaos. **Levers** are the actions a seat can pull
at any moment without being asked. The **scorecard** is the same for every
carrier, seat or not, so the bar to beat is the autopilot running the other
five hundred.

That is the whole design, and the reason it is fun: the world does not
stop for you, the machine is competent, and every choice you make is a
message someone else's system has to handle.

## Departments and what they ask

| Department | The autopilot's rule | What the seat is asked |
| --- | --- | --- |
| **ops** (operations control) | A flight running 46 minutes late or more is announced two hours out as an ASM TIM; an aircraft going technical after check-in is substituted by a smaller type. | *Announce the delay, or hold it* (the sold segments move to TK, or the passengers find out at the airport). *Substitute a smaller aircraft, or cancel.* |
| **crew** | Under 14 CFR 117 a crew that has timed out cancels the flight (code A) unless it leaves the carrier's base, where reserves fly it. | *Cancel, or call reserves* (+90 minutes, a callout paid for) -- asked at T-150, before the cancellation would be announced. |
| **slots** (flow management) | The Network Manager's SAM is taken as given. | *Take the slot, or send REA* -- ready -- and ask for an improvement; the NM answers with an SRM about half the time. |
| **pricing** | The tariff as filed; EMSR-b ladders and network bid prices. | (Levers only for now: the fare multiplier and class overrides. Competitor moves as decisions are on the list.) |
| **ground** (baggage) | Short-shipped bags are rushed on the next flight over the sector. | *Rush them, or hold for tomorrow* (and pay the claims). |

## Levers

| Lever | On the wire |
| --- | --- |
| `cancel` a departure | ASM CNL to every distribution system (and the marketing carrier's for a codeshare), the airport's departure control told, the flight plan withdrawn (CNL to the towers); IROPS reprotects the bookings; the flight plan holds it cancelled so it never departs. |
| `retime` by N minutes | ASM TIM; the systems that sold it move their segments to TK; the flight departs at the new time. |
| `substitute` the aircraft | The cabin rebuilt to a smaller type, re-seated or denied, ASM EQT to distribution. |
| `class` closed or reopened on a flight | An inventory override; AVS goes out as the availability changes. |
| `fares` multiplier | Every fare the carrier files scales from now on, in every distribution system's pricing. |
| `ready` | REA to the Network Manager; an SRM with a better CTOT when the regulation has room. |
| `reserves` | A crew-timed-out cancellation becomes a 90-minute delay with a callout cost, if the announcement has not gone. |

## The scorecard

Per carrier, all day, recomputed on demand:

- **Revenue**: what the day's bookings on its legs were sold for (the
  revenue ledger, per leg, from the priced records).
- **Costs**: block hours by aircraft size, delay minutes past fifteen,
  cancellations per booked passenger, reserve callouts, mishandled bags. The
  numbers are the world's own shape of a cost base and are labelled as such
  in `internal/sim/airline.go`; they are the same for everyone.
- **Punctuality**: departures within fifteen minutes (D15) over flights flown;
  completion; delay minutes.
- **Score** = 100 × margin + 50 × on-time fraction − 2 × cancellations. The
  lobby ranks by it. The autopilot's carriers are on the board too.

## Interfaces

**People**: `/ops/` is the lobby (leaderboard, take a carrier);
`/ops/<code>` the operations centre: scorecard, the inbox with its option
buttons, departments as switches, the levers, the departures board with what
the day has done to each flight (delay in its parts, slot, crew legality,
retimed, substituted, rushed), and a live tape. The carrier's own console --
the jetway node's messages, records, queues -- is one link away.

**Agents**: the same HTTP API, JSON in and out.

```
GET  /carriers.json                      the lobby
POST /carrier/{XX}/take                  {"holder": "name"} → {"seat", "token"}
POST /carrier/{XX}/release               X-Seat-Token
GET  /carrier/{XX}/state                 scorecard, flights, inbox, departments
GET  /carrier/{XX}/inbox
POST /carrier/{XX}/decide                {"id", "option"}          X-Seat-Token
POST /carrier/{XX}/departments           {"department", "manual"}  X-Seat-Token
POST /carrier/{XX}/act                   {"kind", ...}             X-Seat-Token
GET  /carrier/{XX}/events                server-sent events: decisions, actions, incidents
GET  /carrier/{XX}/tape                  the recent events
GET  /dayplan.json                       the weather and the regulations
```

**MCP**: `cmd/skyagent` wraps that API as tools -- `lobby`, `take_seat`,
`carrier_state`, `inbox`, `set_department`, `decide`, `act`, `tape`,
`weather`, `release_seat` -- over stdio, so Claude Code or Claude Desktop can
run a carrier with no code at all:

```json
{ "mcpServers": { "wholesky": { "command": "skyagent", "args": ["-world", "https://wholesky-demo.fly.dev"] } } }
```

Nothing an agent can do is hidden from a person and nothing a person can do
is beyond an agent; a seat can be handed between them mid-day (the token is
the seat).

On the six-machine demo the core runs no carriers: its lobby merges every
region's, and a seat's requests are forwarded to the machine that runs the
carrier. On a single machine everything is local.

## What makes it hard

- The day does not wait. Decisions have real-time deadlines (45 seconds by
  default; `-decision-window` on `skyd`); at warp 6 that is four and a half
  minutes of the day.
- The autopilot is competent. It announces delays, substitutes, rushes bags
  and takes its slots; its scorecard is the baseline, and a seat that only
  answers what it is asked will at best match it.
- Everything costs. Holding a delay announcement saves nothing and strands
  connections; cancelling clears a crew problem and pays for every passenger;
  reserves are cheap on a full flight and dear on an empty one; a fare
  multiplier moves demand you cannot see directly.
- The weather is the same for everyone and the Network Manager does not care
  who you are: a regulation over your hub slots every arrival first come
  first served.
- Protocol is the game. Every lever is a real message another system
  consumes. A retime you announce is a TK on every sold segment in three
  distribution systems, each of which queues a task an agent must work.

## The north star: a multiplayer world

**Many seats, one world** works today: any number of people or agents take
any number of carriers on the same world, and the leaderboard compares them
with each other and with the autopilot.

**Bring your own jetway** is the next step, and it is what keeps jetway the
real thing. A carrier in the world is a jetway node with the world's schedule
and addresses. Nothing in the switch cares where that node runs: the two
switches already identify a link by its hello and route by teletype address,
and the `link_dial` egress (v0.1.69) is a node holding a circuit open to a
switch anywhere on the internet. So:

1. `skyd -external BA` boots the world without BA's tenant. BA's addresses
   stay in the switch's routing table; the GDSes sell into them as before.
2. `GET /carrier/BA/pack` hands a player BA's start pack: the jetway config
   (identity, teletype and AFTN addresses, the switch's address and a link
   token, the distribution systems as peers), BA's schedule as an SSIM file
   (`worldc -ssim`), and the tariff for its markets.
3. The player runs `jetwayd` with that config on their own machine. Their
   node dials the switch; the world's booking traffic arrives on their
   socket; their inventory answers it; their DCS sends the PNL; the world's
   sortation and towers answer them back. The scorecard reads their side
   from the settlement and the messages the switch saw, because that is all
   a real BSP and a real network see.
4. Hard mode: the world's ground story for an external carrier is driven by
   the player's own systems. A missed PNL is a flight the airport never
   opens. The messages have to be right.

**Worlds joined to worlds** is the step after: two `skyd` instances trunk
their switches (`link_dial` with `Trunk: true`, v0.1.71), exchange manifests
over `/federation/register`, and each world's carriers can sell and fly into
the other's airports. Then the regions are continents, the operators are
people, and the sky is whoever showed up.

Steps 1 and 2 are done. `skyd -external BA` boots the world without BA's
tenant and gives BA's switch peer a token; `GET /carrier/BA/pack` returns
the jetway configuration (YAML, ready for `jetwayd -config`), the token, the
switch address, the SSIM file and the notes. The token is jetway v0.1.86's
link token: a hello that names a peer with a token must present it or the
switch refuses the link before any message. `-link-secret` keeps the tokens
stable across restarts; `-public-switch host:port` names the address a
node on the internet dials when it is not the listener's own.

Still missing: the demo exposing its switch port on the internet (a Fly TCP
service; today the pack's switch address is reachable only inside the
demo's network), the scorecard reading an external carrier from the
switch's ledger rather than the tenant's (an external carrier has no
tenant, so the lobby does not yet show it), and the manifest exchange
between worlds. None of it is a new protocol; all of it is jetway doing
what it does across a longer wire.

## Bring your own jetway, locally

```sh
go run ./cmd/skyd -world /tmp/world.json -external BA -link-secret dev -console :8080 &
curl -s localhost:8080/carrier/BA/pack | jq -r .config_yaml > ba.yaml
cd ../jetway && go run ./cmd/jetwayd -config ../wholesky/ba.yaml
```

Your node dials the world's switch as BA; the distribution systems' sells
for BA's flights land on your socket and your inventory answers them.

## Try it locally

```sh
go run ./cmd/worldc -countries "United Kingdom,France" -carriers 6 -o /tmp/world.json
go run ./cmd/skyd -world /tmp/world.json -warp 12 -console :8080
open http://localhost:8080/ops/
# or, as an agent:
go run ./cmd/skyagent -world http://localhost:8080
```

Take a carrier, switch ops and crew to manual, and watch the inbox as the
afternoon banks meet the weather.
