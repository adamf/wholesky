# Security

wholesky runs on the public internet. A core machine serves HTTP on 8080 and
2 Type B switch ports (7000, 7001). Behind it, 5 machines run on the private
network of Fly. A second world joins over the internet. Players, both people
and agents, take seats, claim carriers and point the world at their own
jetway nodes. This page gives the threat model, the findings of 6 auditors
who went through both codebases in September 2026, and the fixes. It also
lists the decisions still open.

## Surfaces and guards

| Surface | Reachable by | Guard |
| --- | --- | --- |
| Lobby, operations centre, the sky, instruments, `/carriers.json`, `/worlds.json`, manifest | anyone | none. These surfaces are public by design. |
| `POST /carrier/XX/take` | anyone | Anyone can take an unheld seat, by design. |
| `release`, `act`, `decide`, `departments`, `claim`, `unclaim`, `node`, `note` | the seat holder | `X-Seat-Token` (96 bits, `crypto/rand`, constant-time compare) |
| `/replay/...`, `/recording/...`, `/recordings.json`, the `recording.json` of a held seat | anyone | none. A run is a public record. The bounds are 20,000 lines per run and 200 runs kept. |
| `POST /federation/world` (another world joining) | anyone | The world vets the hello. It accepts only a public URL and switch address, and applies size caps. It allows 1 attempt per address per 10 s. It limits joins to 16 worlds and 4000 foreign carriers, and refuses address collisions. See the decisions below. |
| `/federation/register`, `/federation/token`, `/federation/state/{peer}`, `/federation/recording/{id}`, `POST /shard/*` | the machines of this world | `X-Skyd-Secret` = `SKYD_LINK_SECRET`, compared in constant time. Any other request gets 403. |
| `POST /eye/time`, `POST /fleet/node/XX/link` | the operator | the same secret. The operator enters it once on the page, and the browser keeps it. |
| `POST /eye/chaos` | anyone | 1 act per address per 30 s. See the decisions below. |
| `/node/XX/...` (the jetway console of a carrier) | anyone reads. The seat holder books, cancels and boards. | `X-Seat-Token` for every method except GET. The console page reads the token from the browser storage of the operations centre. `/api/admin/` is never reachable. |
| TCP 7000/7001 (switch links) | anyone | A hello must carry a token that is configured on the switch. The switch refuses a name without a token (jetway `require_token`). |
| pprof | nobody | bound to loopback only. It refuses any other address. |

## Findings and fixes (September 2026)

The findings below were verified live or by test, unless marked inferred.
The jetway fixes shipped in v0.1.94. The wholesky fixes are in the commit
that added this page.

**Critical**

- `GET /federation/state/{peer}` returned the saved state of a region to
  anyone. The state included every seat token and the trunk tokens of the
  joined worlds. `PUT` overwrote the state. A stranger could take over every
  carrier in play. The endpoint is now behind the link secret. The state file
  is `0600`, and the peer table is bounded.
- `POST /federation/register` let anyone name a URL. The core then polled
  that URL, proxied player requests to it with their seat tokens, and took
  instrument data from it. `POST /federation/token` let anyone cut or take
  the switch link of any carrier. Both endpoints are now behind the link
  secret.
- The switch ports accepted any hello for a name that had no token, and
  every internal carrier had no token. The new session displaced the live
  session. A stranger could sit in the middle of the traffic of a carrier.
  The `require_token` option of jetway now refuses a name without a token.
  Every peer of the public switches carries a token derived from the link
  secret.
- The node console proxy forwarded every jetway route. Anyone could book,
  cancel, refund, board and export the records of any carrier. Now a
  stranger can only read. The seat holder of the carrier works the console
  as the airline, and the page carries the seat token. Nobody reaches the
  admin surface. The jetway console also gained an optional
  `http.admin_token` for nodes that people run themselves.
- The world controls were open. These controls stop the clock and sever the
  link of a carrier. Now only the operator can use them. The weather act, an
  airport closure, stays public, limited to 1 per address per 30 s. See the
  decisions below.

**High**

- `/ops/{carrier}` had a reflected cross-site scripting (XSS) flaw. The
  carrier code went into the markup unescaped, and seat tokens live in the
  browser storage. The code is now validated (`[A-Z0-9]{2,3}`) and escaped.
  The pages carry a content security policy and `nosniff`.
- Server-side request forgery was possible. The core fetched the node URL of
  a player, and fetched or dialled the URL and switch address of another
  world, without validation. The targets could be loopback, the private
  network of Fly, or cloud metadata. Both must now resolve to public
  addresses, and `-allow-private-peers` allows private addresses for a test
  on 1 machine. The core does not follow redirects. Every response read is
  bounded: 48 MB for a manifest, 4 MB for a lobby, 32 MB for a revenue feed.
- Names supplied by strangers caused unbounded growth in peer-state keys,
  joined worlds and their carriers, and event-stream subscription tables. On
  the jetway console, `?limit=` sized an allocation, and 1 request could
  allocate 800 MB. All of these are now bounded.
- The switch ports had no idle reaping, no connection cap and no context
  close. Idle sockets held goroutines for ever. jetway listeners now have
  `idle_timeout` and `max_connections` (4096), and they close with their
  context.
- Tokens cross TCP 7000/7001 in plaintext. See the decisions below.

**Medium and low**

- The HTTP servers of the wholesky regions and the jetway console had no
  read, idle or header bounds. They are now set. Event streams are capped at
  256 per server.
- Fuzzing every wire and file parser in jetway found 2 parser panics. One
  was a New Distribution Capability (NDC) SOAP body on invalid UTF-8, and the
  other an empty name item. Both are fixed. The crashers are kept as
  regression seeds, and 21 fuzz harnesses were added.
- Sentinel framing buffered a whole stream before it checked the bound. It
  now checks while it reads.
- Fare multipliers accepted `1e308`. They are now clamped to 0.1–10.
- Some token comparisons were not constant-time. All of them are now.
- The regions ran out of memory every few hours. The books of 200 carriers
  kept every record and its history until the day wrapped. The settlement
  kept the HOT file of every airline. Flown records now leave 3 hours after
  their last departure (jetway `store.Pruner`). Availability beliefs are
  purged as flights depart, and HOT files are built on request.
- `govulncheck` is clean on both repositories, and `go mod verify` is clean.
  The containers run as non-root, with no secrets in the image. Only the
  core has public services on Fly.

## Decisions still open

The author must make these decisions. They are not patches.

1. **TLS on the switch ports.** Tokens and Type B traffic cross the internet
   in the clear on 7000/7001. jetway supports TLS and mutual TLS on every
   listener. Turning it on changes how a player sets up their node, which
   then needs a certificate in addition to the token from the pack. The
   Kubernetes layout (`deploy/k8s`) terminates TLS for HTTP at its Envoy edge
   and passes the switch ports through as raw TCP, as today.
2. **Open or invited joins.** A join is open by design. The mirror world
   joins the demo with no prior arrangement. The world now vets and paces
   joins. A hostile world can still bring thousands of fake carriers into
   the lobby and route messages to them. An invitation secret exchanged out
   of band would close this gap.
3. **Booking from the public console.** The jetway console is
   unauthenticated by design, behind the network of an operator. The proxy
   of the world makes it read-only. A node that someone runs on the internet
   should set `http.admin_token`. The start pack could carry one.
4. **Anonymous weather.** Closing an airport cancels every flight that
   touches it, for every player. The act is rate-limited. It is not removed.
5. **Unsigned Type B origin.** A joined world can send a movement or a
   cancellation for any carrier. Members of the industry Type B network can
   do the same. On a public federation, binding the origin of a message to
   the link that it arrived on would stop this. That would also block
   legitimate multi-address traffic.
6. **Relay loops across 3 or more worlds.** A message cannot loop between 2
   worlds. It can loop around a ring of 3 worlds. The fix is a hop count or
   a seen-set in the switch. The choice between them is a protocol choice.
7. **Storage of seat tokens.** Regions keep seat tokens at the core, because
   the free tier allows 1 volume. An alternative keeps them on the machine
   that owns them and never round-trips them.

## Reporting

Open an issue at github.com/adamf/wholesky or github.com/adamf/jetway, or
email the author. All passenger data in a demo world is synthetic.
