# Run a carrier

wholesky runs 518 carriers on autopilot. This page describes how a person or
an agent takes a carrier. It also describes where that leads: many people
running many carriers, players running their own jetway nodes, and worlds
joined to other worlds.

The demo lobby is at **https://wholesky-demo.fly.dev/ops/**. The operations
centre of a carrier is at `/ops/<code>`. The same functions are available as
Model Context Protocol (MCP) tools:
`go run ./cmd/skyagent -world https://wholesky-demo.fly.dev`.

## The idea

Every carrier in the world has the systems of an airline:

- a reservations host that answers the distribution systems over Type B and
  EDIFACT
- a seat inventory under revenue management
- a departure control system (DCS) at every airport that the carrier serves
- an operations desk that files flight plans and reads the messages from the
  towers
- a bag office
- a settlement position at the end of the day

The autopilot runs those systems. The autopilot is a set of rules in
`internal/host` and `internal/sim`. At every point where an airline has a
person deciding, the rules decide what the carrier does.

A **seat** is a person or an agent who takes those decisions instead of the
autopilot. Taking a seat changes nothing by itself. Every **department** stays
on autopilot until the seat sets it to manual. When a department is manual,
the simulator stops deciding and sends a **decision** to the seat. The
decision appears in the inbox of the seat with the situation, the options,
the cost of each option, a default and a deadline. The simulator applies the
answer and sends the resulting messages to the other systems.

If the seat does not answer before the deadline, the simulator applies the
default. A slow player therefore gets the results of the autopilot. **Levers**
are actions that a seat can take at any moment, without a decision from the
simulator. The **scorecard** uses the same formula for every carrier, with or
without a seat. A seat competes against the autopilot, which runs every other
carrier.

That is the whole design. The world does not stop for a seat. The autopilot
is competent. Every choice that a seat makes becomes a message that another
system must handle.

## Departments and decisions

| Department | Autopilot rule | Decision sent to the seat |
| --- | --- | --- |
| **ops** (operations control) | When a flight runs 46 minutes late or more, the autopilot announces the delay 2 hours before departure. The announcement is an ad hoc schedule message (ASM) of type TIM. When an aircraft goes technical after check-in, the autopilot substitutes a smaller type. | *Announce the delay, or hold it.* An announced delay moves the sold segments to TK. With a held delay, the passengers learn of it at the airport. *Substitute a smaller aircraft, or cancel.* |
| **crew** | Under 14 CFR 117, a crew that has timed out cancels the flight (code A). If the flight leaves the base of the carrier, reserves fly it instead. | *Cancel, or call reserves.* Reserves add 90 minutes and a paid callout. The simulator sends this decision at T-150, before the cancellation would be announced. |
| **slots** (flow management) | The autopilot accepts the slot allocation message (SAM) from the Network Manager. | *Take the slot, or send a ready message (REA)* to ask for an improvement. The Network Manager answers with a slot revision message (SRM) about half the time. |
| **pricing** | The autopilot sells the tariff as filed, with EMSR-b ladders and network bid prices. It does not change its fares. | About once an hour of the day: *a rival has cut fares on one of your markets. Match, or hold.* Matching lowers the multiplier of that market. Travellers compare fares. Fares above the filing lose buyers. The elasticity is 1.5, a parameter of the world. Fares below the filing sell every seat that they can. |
| **ground** (baggage) | The autopilot rushes short-shipped bags on the next flight over the sector. | *Rush the bags, or hold them for tomorrow.* If the bags are held, the carrier pays the claims. |

## Levers

| Lever | Effect |
| --- | --- |
| `cancel` a departure | The carrier sends ASM CNL to every distribution system, and to the marketing carrier for a codeshare. It tells the DCS at the airport and withdraws the flight plan with CNL to the towers. Irregular operations (IROPS) reprotects the bookings. The flight plan holds the flight cancelled, and the flight never departs. |
| `retime` by N minutes | The carrier sends ASM TIM. The systems that sold the flight move their segments to TK. The flight departs at the new time. |
| `substitute` the aircraft | The carrier rebuilds the cabin to a smaller type and re-seats or denies passengers. It sends ASM EQT to the distribution systems. |
| `class` closed or reopened on a flight | The carrier overrides the inventory. It sends availability status (AVS) messages as the availability changes. |
| `fares` multiplier, optionally a market (`from`, `to`) | From now on, the multiplier scales every fare that the carrier files, in the pricing of every distribution system. With a market, it scales only the fares of that market. Above the filing, fewer travellers buy. |
| `ready` | The carrier sends REA to the Network Manager. When the regulation has room, the Network Manager answers with an SRM that has a better calculated take-off time (CTOT). |
| `reserves` | If the cancellation has not been announced, a cancellation for a timed-out crew becomes a 90-minute delay with a callout cost. |

## The scorecard

The simulator computes the scorecard per carrier, over the whole day, on
demand:

- **Revenue**: the sale value of the bookings of the day on the legs of the
  carrier. The source is the revenue ledger, per leg, from the priced records.
- **Costs**: block hours by aircraft size, delay minutes past 15, cancellations
  per booked passenger, reserve callouts, and mishandled bags. The cost rates
  are a cost base chosen for the world, and `internal/sim/airline.go` labels
  them as such. They are the same for every carrier.
- **Punctuality**: departures within 15 minutes (D15) as a fraction of flights
  flown, completion, and delay minutes.
- **Score** = 100 × margin + 50 × on-time fraction − 2 × cancellations. The
  lobby ranks all carriers by score, including the carriers on autopilot.

## Interfaces

**People**: `/ops/` is the lobby. It shows the leaderboard and lets you take a
carrier. `/ops/<code>` is the operations centre. It shows the scorecard, the
inbox with its option buttons, a toggle for each department, the levers, the
departures board and a live tape. The departures board shows the state of each
flight. This includes the parts of its delay, its slot, its crew legality, and
whether it was retimed, substituted or rushed. A link on the page opens the
console of the carrier, with the messages, records and queues of its jetway
node.

**Agents**: agents use the same HTTP API, with JSON requests and responses.

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

**MCP**: `cmd/skyagent` wraps that API as MCP tools over stdio. The tools are
`lobby`, `take_seat`, `carrier_state`, `inbox`, `set_department`, `decide`,
`act`, `tape`, `weather` and `release_seat`. Claude Code or Claude Desktop can
then run a carrier with no code:

```json
{ "mcpServers": { "wholesky": { "command": "skyagent", "args": ["-world", "https://wholesky-demo.fly.dev"] } } }
```

A person and an agent have the same capabilities. The seat can move between
a person and an agent during the day. Whoever holds the seat token holds the
seat.

On the 6-machine demo, the core runs no carriers. The core lobby merges the
lobbies of every region. The core forwards a seat's requests to the
machine that runs the carrier. On a single machine, everything is local.

## Sources of difficulty

- The world does not wait for a decision. Decisions have wall-clock deadlines.
  The default deadline is 45 seconds, set by `-decision-window` on `skyd`. At
  warp 6, 45 seconds is 4.5 minutes of the day.
- The autopilot is competent. It announces delays, substitutes aircraft,
  rushes bags and takes its slots. Its scorecard is the baseline. A seat that
  only answers the decisions it receives can at best match the autopilot.
- Every action has a cost. Holding a delay announcement saves nothing and
  strands connections. Cancelling clears a crew problem, but the carrier pays
  for every passenger. Reserves are cheap on a full flight and expensive on
  an empty one. A fare multiplier moves demand that the seat cannot see
  directly.
- The weather is the same for every carrier, and the Network Manager treats
  every carrier the same. A regulation over your hub gives slots to every
  arrival on a first-come, first-served basis.
- Every lever sends a message that another system consumes. An announced
  retime becomes a TK on every sold segment in 3 distribution systems. Each
  distribution system then queues a task that an agent must work.

## A multiplayer world

**Many seats, one world** works today. Any number of people or agents take
any number of carriers on the same world. The leaderboard compares them with
each other and with the autopilot.

**Bring your own jetway** is the next step. It keeps jetway usable outside
the simulation. A carrier in the world is a jetway node with the schedule
and addresses of the world. The switch does not depend on where that node
runs. The two switches identify a link by its hello and route by teletype
address. The `link_dial` egress (v0.1.69) is a node that holds a circuit
open to a switch anywhere on the internet. The steps are:

1. `skyd -external BA` boots the world without the tenant of BA. The
   addresses of BA stay in the routing table of the switch. The distribution
   systems sell into them as before.
2. `GET /carrier/BA/pack` gives a player the start pack of BA. The pack
   contains the jetway configuration, the schedule of BA as a Standard
   Schedules Information Manual (SSIM) file (`worldc -ssim`), and the tariff
   for its markets. The configuration contains the identity and the teletype
   and Aeronautical Fixed Telecommunication Network (AFTN) addresses. It also
   contains the address of the switch, a link token, and the distribution
   systems as peers.
3. The player runs `jetwayd` with that configuration on their own machine.
   Their node dials the switch. The booking traffic of the world arrives on
   their socket, and their inventory answers it. Their DCS sends the
   passenger name list (PNL). The sortation and towers of the world answer
   them. The scorecard reads their side from the settlement and from the
   messages that the switch saw. A Billing and Settlement Plan (BSP) and a
   network see only those sources.
4. In hard mode, the systems of the player drive the ground handling of the
   world for an external carrier. If the player misses a PNL, the airport
   never opens the flight. The messages must be correct.

**Worlds joined to worlds** works. It makes 1 network from 2 `skyd`
instances, each with its own carriers, distribution systems, switch and day.
The command for the second instance names the address of the first:

```sh
skyd -world eu.json  -world-name europe  -console :8080 -public-url http://eu.example:8080 -link-port 7000
skyd -world am.json  -world-name americas -world-code 1Z -world-city MIA -console :8081 -public-url http://am.example:8081 -link-port 7100 -peer-world http://eu.example:8080
```

The handshake (`POST /federation/world`) carries what each side needs. It
carries the designator and address of the switch, a token for the trunk,
and the address that the globe watches. It also carries the carriers and
distribution systems of the side, and the location of its manifest. Each
side adds the switch of the other side as a trunk. One side accepts the
trunk and the other side dials it. jetway v0.1.90 dials a link that was
added while it runs.

Each side then routes the carriers and distribution systems of the other
side down the trunk, and fetches the manifest of the other side. Its own
distribution systems then sell the flights of the other side, and it prices
those markets. It also instructs its carriers to copy their movements to the
globe of the other side. The traffic over the trunk is Type B and EDIFACT,
the same as within a single world.

The worlds must differ in 3 things, because the network uses them as
addresses. These are the designators of the carriers, the codes of the
switches (`-world-code`), and the cities of the distribution systems
(`-world-city`). The world refuses a join where the designators overlap. A
test with 2 compiled worlds in 1 process covers this path. The trunk comes up
in both directions. A seat that the distribution system of each world sells
on a carrier of the other world lands in the book of that carrier.

The mirror world at **https://wholesky-mirror.fly.dev** is a second world
that trunks to the demo at boot. It has the same data as the demo, with
every carrier renamed by `worldc -mirror Q`. The renamed designators collide
with no other carrier. Its carriers appear in the demo lobby under *mirror*.
The distribution systems of the demo sell its flights over the trunk, and
its aircraft fly on the demo globe. The release gate boots a small mirror
beside the small world and waits for the join.

In that design, each region is a continent, and different people operate
the regions. Anyone who connects a node takes part.

Steps 1 to 3 work, including on the demo. There are 2 ways to start:

- **Claim a running carrier.** Take the seat, then send
  `POST /carrier/BA/claim` with the seat token. The world severs the tenant
  of BA from the switch. Every switch then demands the link token of BA on the
  hello. jetway v0.1.87 sets the token at runtime and cuts the old link. The
  response is the start pack. It contains the jetway configuration as YAML for
  `jetwayd -config`, the token and the switch address. It also contains the
  schedule of BA as an SSIM file, and the notes. `POST /carrier/BA/unclaim`
  returns the carrier to the world.
- **Boot without the carrier.** `skyd -external BA` never boots the tenant
  of BA.

The demo switches listen on the internet. The first switch is at
`wholesky-demo.fly.dev:7000` and the second at `:7001`. The pack names the
switch that homes your carrier. The link secret is a Fly secret, and a token
therefore survives a restart. The token check in jetway v0.1.86 refuses a
node that names a carrier without its token, before any message.

Your node is a carrier as well as a gateway. The `ops:` block of the pack
(jetway v0.1.88, `pkg/ops`) gives it an operations desk. The desk reads the
schedule from the SSIM file beside the configuration. The DCS at your
stations opens flights from your own name lists. The datalink provider of
the world sends out-off-on-in (OOOI) reports from the aircraft. The desk
converts them into the movement messages (MVT) that the globe draws. It
files the messages from the towers and the Network Manager against the
callsign.

The world runs the network side of your day: the slot 2 hours before
departure, the datalink reports, and the towers. You run the carrier side
from the console or the API of the node. This side includes sending the
PNL, checking in, closing the door, and announcing your own cancellations.

`internal/sim/byo_test.go` tests the whole path end to end. A jetway node
built only from the YAML of the pack dials the switch of the small world and
comes up as the carrier. A seat that the distribution system of the world
sells on one of its flights lands in the book of the node. When the datalink
of the world reports the departure, the MVT of the node reaches the
distribution system.

The book of a claimed carrier is on your node. Its scorecard therefore reads
the sales of the distribution systems on its legs: money and passengers,
from their ledgers, federated by the core.

The carriers of joined worlds appear on the leaderboards of each other,
labelled with their world. The world that runs a carrier answers the
a seat's requests for it. A join made at a federated core reaches its
peers. The machines of the distribution systems take the carriers and
flights of the other world to sell. The tenants of the regions copy their
movements to the globe of the other world.

If you prefer to watch rather than work the counter, tell the world where
your node answers. Send `POST /carrier/BA/node {"url": "http://your-node:8080"}`
with the seat token. The URL must be reachable from the internet. The world
does not fetch from its own network at the request of a stranger. It
refuses loopback addresses, private ranges and the `.internal` names of
Fly. A world started with `-allow-private-peers`, as in a test on 1
machine, allows them.

The world then keeps the ground handling hours of your carrier, as it does for
its own tenants. It requests the name list from the desk of your node 3 hours
before each departure (`POST /api/ops/flight/BA0117/26NOV/LHR/pnl`, jetway
v0.1.93). It requests the rest 45 minutes before departure (`.../run`). The
rest is every passenger accepted with a bag, the counter closed and the cabin
boarded. It ends with the door closed with the load, and the closure messages
sent. Send `{"url": ""}` to clear the URL. You then run the hours again.

None of this is a new protocol. jetway does the same work as before, over
the internet.

## Bring your own jetway

Run this against the demo, with the seat token from `/ops/`:

```sh
curl -s -X POST -H "X-Seat-Token: $TOKEN" https://wholesky-demo.fly.dev/carrier/BA/claim | jq -r .config_yaml > ba.yaml
go run github.com/adamf/jetway/cmd/jetwayd@latest -config ba.yaml     # or `skyagent`'s claim tool
```

For a local run, add `-allow-private-peers`, because your node is on
loopback:

```sh
go run ./cmd/skyd -world /tmp/world.json -external BA -link-secret dev -allow-private-peers -console :8080 &
curl -s localhost:8080/carrier/BA/pack | jq -r .config_yaml > ba.yaml
cd ../jetway && go run ./cmd/jetwayd -config ../wholesky/ba.yaml
```

Your node dials the switch of the world as BA. The sells of the distribution
systems for the flights of BA land on your socket, and your inventory
answers them.

## A local world

```sh
go run ./cmd/worldc -countries "United Kingdom,France" -carriers 6 -o /tmp/world.json
go run ./cmd/skyd -world /tmp/world.json -warp 12 -console :8080
open http://localhost:8080/ops/
# or, as an agent:
go run ./cmd/skyagent -world http://localhost:8080
```

Take a carrier, set ops and crew to manual, and watch the inbox when the
afternoon banks meet the weather.

## Recordings

The simulator records every run from take to release. The recording
contains each decision that the simulator sent, and each answer of the seat
with its time. It also contains each lever that the seat used, the changes
to the scorecard, and the notes of the seat. Every line carries the
simulation clock.

While a seat is held, `/replay/BA` plays the run so far. When the seat is
released, the world keeps the run, and `/replay/<id>` plays it permanently.
The lobby links the last run of each seat, and `/recordings.json` lists all
runs.

The seat writes its notes with `POST /carrier/BA/note {"text": "..."}`
and the seat token. `skyagent` exposes this as the `note` tool and instructs
the agent to narrate. A replay of the day of an agent therefore shows the
reasoning of the agent beside the tape. `skyagent -record run.jsonl` also
keeps every call and answer locally. A replay page also accepts `?src=`, and
a recording copied to a static site plays there without the world.

## Recording an agent's day

The commands below made the run on the site (wholesky.io/replay). Use the
same commands for any run:

```sh
go run ./cmd/worldc -countries "United Kingdom,Ireland,Spain,Portugal,France" -carriers 8 -o west.json
go run ./cmd/skyd -world west.json -carriers 0 -warp 60 -fill 0.8 -demand 300 \
    -link-secret demo -allow-private-peers -state ./state.json -console 127.0.0.1:8095
# the books take a few minutes to fill; hold the clock meanwhile, then let it go
curl -X POST -H "X-Skyd-Secret: demo" -d '{"warp":0}'  localhost:8095/eye/time
curl -X POST -H "X-Skyd-Secret: demo" -d '{"warp":60}' localhost:8095/eye/time
# the agent: Claude Code in print mode with skyagent as its MCP server
claude -p "$(cat prompt.md)" --mcp-config mcp.json --strict-mcp-config \
    --allowedTools mcp__wholesky__lobby,mcp__wholesky__take_seat,mcp__wholesky__inbox,mcp__wholesky__decide,mcp__wholesky__note,... \
    --output-format stream-json --verbose --max-turns 320
```

`mcp.json` points `skyagent -world http://127.0.0.1:8095 -record run.jsonl`
at the world. The prompt instructs the agent to take a carrier and set every
department to manual. It also instructs the agent to answer the inbox until
the clock passes 23:00, narrate with `note`, and release the seat. After the
release, the run is on the disk of the core, and `/replay/<id>` plays it.
`curl /recording/<id>.json` returns the file that the copy on the site was
made from.

`asciinema rec --headless` around the `claude` command records the side of
the agent. `docs/replay/terminal.html` plays that recording.
`docs/replay/day.html` plays the terminal and the replay together.

The world ran at 1 simulation hour per minute from the second that the
terminal recording began. Each second of the terminal is therefore 1 minute
of the day. `?t=SECONDS` renders 1 still frame. Headless Chrome rendered
these stills frame by frame to make the video.

## Access control

Anyone can read the lobby, the operations centre, the sky and the console
of every carrier. Every change needs a credential. A change to a carrier
needs its seat token. The control plane between machines and the controls
of the operator need the link secret. The controls of the operator are the
clock and the cutting of a circuit. A link on 7000/7001 needs a token that
is configured on the switch.

The consoles of the carriers at `/node/XX/` are read-only for everyone
except the seat holder. When you take a carrier, you can book, cancel and
board from its console, as the airline. The threat model, the audit that
shaped it and the decisions still open are in [security.md](security.md).
