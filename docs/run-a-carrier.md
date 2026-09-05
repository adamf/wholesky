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

**Worlds joined to worlds** works. Two `skyd` instances -- two skies, each
with its own carriers, distribution systems, switch and day -- become one
network the way two real networks do. The second is told where the first
is:

```sh
skyd -world eu.json  -world-name europe  -console :8080 -public-url http://eu.example:8080 -link-port 7000
skyd -world am.json  -world-name americas -world-code 1Z -world-city MIA -console :8081 -public-url http://am.example:8081 -link-port 7100 -peer-world http://eu.example:8080
```

The handshake (`POST /federation/world`) carries what each side needs: its
switch's designator and address, a token for the trunk, the address its
globe watches, its carriers and distribution systems, and where its
manifest is. Each side adds the other's switch as a trunk (one accepts, one
dials, jetway v0.1.90 dialling a link added while running), routes the
other's carriers and distribution systems down it, fetches the other's
manifest so its own distribution systems sell the other's flights, prices
those markets, and tells its carriers to copy their movements to the
other's globe. The traffic itself is Type B and EDIFACT over the trunk,
exactly as within one world.

Three things must differ between worlds, because in this network they are
addresses: the carriers' designators (the join is refused where they
overlap), the switches' codes (`-world-code`), and the distribution
systems' cities (`-world-city`). Tested with two compiled worlds in one
process: the trunk comes up both ways and a seat sold by each world's
distribution system on the other's carrier lands in that carrier's book.

Then the regions are continents, the operators are people, and the sky is
whoever showed up.

Steps 1 to 3 work, and on the demo. Two ways in:

- **Claim a running carrier.** Take the seat, then
  `POST /carrier/BA/claim` with the seat's token. The world severs BA's
  tenant from the switch, every switch starts demanding BA's link token on
  the hello (jetway v0.1.87 sets it at runtime and cuts the old link), and
  the start pack comes back: the jetway configuration (YAML, ready for
  `jetwayd -config`), the token, the switch address, BA's schedule as an
  SSIM file, and the notes. `POST /carrier/BA/unclaim` gives it back.
- **Boot without it.** `skyd -external BA` never boots BA's tenant.

The demo's switches listen on the internet: `wholesky-demo.fly.dev:7000`
(the first switch) and `:7001` (the second); the pack names the one that
homes your carrier. The link secret is a Fly secret, so a token survives a
restart. jetway v0.1.86's token check means a node that names a carrier
without its token is refused before any message.

Your node is a carrier, not only a gateway. The pack's `ops:` block
(jetway v0.1.88, `pkg/ops`) gives it an operations desk: the schedule from
the SSIM file beside the configuration, departure control at your stations
opening flights from your own name lists, the aircraft's OOOI reports from
the world's datalink provider turned into the MVTs the globe draws, the
towers' and the Network Manager's messages filed against the callsign. The
world keeps flying the network side of your day -- the slot two hours out,
the datalink reports, the towers -- and leaves the carrier's side to you:
sending the PNL, checking in, closing the door, announcing your own
cancellations, from the node's console or its API.

The whole path is tested end to end in `internal/sim/byo_test.go`: a
jetway node built from nothing but the pack's YAML dials the small world's
switch, comes up as the carrier, a seat the world's distribution system
sells on one of its flights lands in the node's own book, and when the
world's datalink reports the departure the node's MVT reaches the
distribution system.

Still missing: an external carrier's scorecard from the switch's ledger
(the lobby shows a claimed carrier, but its passengers are now in your
book, not the world's), a timetable that drives your node's ground story
for you if you want the autopilot's help, joined worlds' carriers on each
other's leaderboards, and the join from a federated core to its regions'
tenants (today a joined world's watcher reaches the tenants on the machine
that holds the switch). None of it is a new protocol; all of it is jetway
doing what it does across a longer wire.

## Bring your own jetway

Against the demo, with the seat's token from `/ops/`:

```sh
curl -s -X POST -H "X-Seat-Token: $TOKEN" https://wholesky-demo.fly.dev/carrier/BA/claim | jq -r .config_yaml > ba.yaml
go run github.com/adamf/jetway/cmd/jetwayd@latest -config ba.yaml     # or `skyagent`'s claim tool
```

Locally:

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
