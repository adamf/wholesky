# Security: what a stranger can reach, and what they get

wholesky runs on the public internet: a core machine serving HTTP on 8080
and two Type B switch ports (7000, 7001), five machines behind it on Fly's
private network, a second world joined over the internet, and players --
people and agents -- taking seats, claiming carriers and pointing the
world at their own jetway nodes. This page is the threat model, what was
found when six auditors went through both codebases in September 2026,
what was fixed, and what is left as a decision.

## Who can reach what

| Surface | Reachable by | Guard |
| --- | --- | --- |
| Lobby, ops centre, the sky, instruments, `/carriers.json`, `/worlds.json`, manifest | anyone | none: they are the public face |
| `POST /carrier/XX/take` | anyone | an unheld seat is anyone's to take, by design |
| `release`, `act`, `decide`, `departments`, `claim`, `unclaim`, `node`, `note` | the seat's holder | `X-Seat-Token` (96 bits, `crypto/rand`, constant-time compare) |
| `/replay/...`, `/recording/...`, `/recordings.json`, a held seat's `recording.json` | anyone | none: a run is a public record (bounded: 20,000 lines, 200 kept) |
| `POST /federation/world` (another world joining) | anyone | the hello is vetted (public URL and switch address only, size caps, one attempt per address per 10 s, 16 worlds, 4000 foreign carriers, no address collisions); see decisions below |
| `/federation/register`, `/federation/token`, `/federation/state/{peer}`, `/federation/recording/{id}`, `POST /shard/*` | this world's own machines | `X-Skyd-Secret` = `SKYD_LINK_SECRET`, constant-time; 403 otherwise |
| `POST /eye/time`, `POST /fleet/node/XX/link` | the operator | the same secret, entered once on the page (kept in the browser) |
| `POST /eye/chaos` | anyone | one act per address per 30 s; see decisions below |
| `/node/XX/...` (a carrier's jetway console) | anyone reads; the seat's holder books, cancels, boards | `X-Seat-Token` for anything but GET (the console page picks it up from the ops centre's browser storage); never `/api/admin/` |
| TCP 7000/7001 (switch links) | anyone | a hello must carry a token the switch knows; tokenless names are refused (jetway `require_token`) |
| pprof | nobody | bound to loopback only, refuses any other address |

## What was found and fixed (September 2026)

Verified live or by test unless marked inferred. jetway fixes shipped in
v0.1.94; wholesky's in the commit that added this page.

**Critical**

- `GET /federation/state/{peer}` returned a region's saved state to anyone,
  including every seat's token and the joined worlds' trunk tokens; `PUT`
  overwrote it. A stranger could take over every carrier being played.
  Now behind the world's secret; the state file is `0600`; the peer table
  is bounded.
- `POST /federation/register` let anyone name a URL the core would then
  poll, proxy player requests to (with their seat tokens), and take
  instrument data from. `POST /federation/token` let anyone cut or take
  any carrier's switch link. Both behind the secret now.
- The switch ports took any hello for a name that had no token -- every
  internal carrier -- and displaced the live session: a stranger could sit
  in the middle of a carrier's traffic. jetway's `require_token` refuses
  tokenless names; every peer of the public switches carries a token
  derived from the world's secret.
- The node console proxy forwarded every jetway route, so anyone could
  book, cancel, refund, board and export any carrier's records. Now a
  stranger reads; the carrier's seat holder works the console as the
  airline (the page carries the seat's token); nobody reaches the admin
  surface. jetway's console also gained an optional `http.admin_token`
  for nodes people run themselves.
- World controls -- stop the clock, sever a carrier's link -- were open.
  Now the operator's. Weather (airport closures) stays a public act, one
  per address per half-minute (a decision, below).

**High**

- Reflected XSS in `/ops/{carrier}`: the code went into the markup
  unescaped; seat tokens live in the browser's storage. The code is
  validated (`[A-Z0-9]{2,3}`) and escaped; the pages carry a content
  security policy and `nosniff`.
- Server-side request forgery: a player's node URL and another world's
  URL and switch address were fetched or dialled unvalidated -- loopback,
  Fly's private network, cloud metadata. Both must now resolve to public
  addresses (`-allow-private-peers` for a test on one machine), redirects
  are not followed, and every response read is bounded (48 MB manifest,
  4 MB lobby, 32 MB revenue feed).
- Unbounded growth from strangers' names: peer-state keys, joined worlds
  and their carriers, event-stream subscription tables, `?limit=` sizing
  an allocation on the jetway console (a single request could allocate
  800 MB). All bounded.
- No idle reaping, connection cap or context close on the switch ports:
  idle sockets held goroutines for ever. jetway listeners now have
  `idle_timeout`, `max_connections` (4096) and close with their context.
- Plaintext tokens on TCP 7000/7001 (a decision, below).

**Medium and low**

- HTTP servers without read, idle or header bounds (wholesky's regions,
  jetway's console): set. Event streams capped at 256 per server.
- Two parser panics found by fuzzing every wire and file parser in
  jetway (NDC SOAP body on invalid UTF-8; empty name item): fixed, with
  the crashers kept as regression seeds and 21 fuzz harnesses added.
- Sentinel framing buffered a whole stream before checking its bound:
  checks while reading now.
- Fare multipliers accepted `1e308`: clamped to 0.1–10.
- Token comparisons that were not constant-time: they are.
- Memory: the regions ran out of memory every few hours. Two hundred
  carriers' books kept every record and its history until the day
  wrapped, and the settlement kept every airline's HOT file. Flown records
  now leave three hours after their last departure (jetway
  `store.Pruner`), availability beliefs are purged as flights go, and HOT
  files are built when asked for.
- `govulncheck` clean on both repos; `go mod verify` clean; containers run
  as non-root with no secrets in the image; only the core has public
  services on Fly.

## Decisions still open

These need the author's call, not a patch.

1. **Should the switch ports carry TLS?** Tokens and Type B traffic cross
   the internet in the clear on 7000/7001. jetway supports TLS and mutual
   TLS on every listener; turning it on changes how a player's node is
   set up (a certificate, not just a token from the pack). The Kubernetes
   layout (`deploy/k8s`) terminates TLS for HTTP at its Envoy edge and
   passes the switch ports through as the raw TCP they are today.
2. **Should any world be able to join, or only invited ones?** A join is
   open by design (the mirror joins the demo with no prior arrangement);
   it is vetted and paced now, but a hostile world can still bring
   thousands of fake carriers into the lobby and route messages to them.
   An invitation secret exchanged out of band would close that.
3. **Should the public console be able to book at all?** jetway's console
   is unauthenticated by design behind an operator's network; the world's
   proxy makes it read-only. A node someone runs on the internet should
   set `http.admin_token`; the start pack could carry one.
4. **Is anonymous weather a feature?** Closing an airport cancels every
   flight touching it, for every player. It is rate-limited, not removed.
5. **Type B origin is unsigned.** A joined world can send a movement or a
   cancellation for any carrier, as the real network's members can; on a
   public federation, binding a message's origin to the link it arrived on
   would stop it, at the cost of legitimate multi-address traffic.
6. **Relay loops across three or more worlds.** Two worlds cannot
   ping-pong a message; a ring of three could. A hop count or a seen-set
   in the switch is the fix, and which one is a protocol choice.
7. **Where seats live.** Regions keep seat tokens at the core (the free
   tier allows one volume); an alternative keeps them on the machine that
   owns them and never round-trips them.

## Reporting

Open an issue at github.com/adamf/wholesky or github.com/adamf/jetway, or
email the author. Nothing in a demo world is real passenger data.
