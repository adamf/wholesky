#!/usr/bin/env sh
# The load test as a release gate: boot a world on real sockets at warp 240,
# let the departure banks run, then ask it whether its laws held. Exit
# non-zero if a cabin holds more than it has, a shard did not answer, or
# nothing flew or sold -- a quiet sky is a failure too. The day's position
# is anchored to the wall clock, so the run must be long enough to cross the
# night: at warp 240 the eight quiet hours are two minutes.
#
#   scripts/gate.sh world.json [seconds]
set -eu
WORLD=${1:-world.json}
SECS=${2:-200}
go build -o /tmp/skyd ./cmd/skyd
go build -o /tmp/skycheck ./cmd/skycheck
/tmp/skyd -world "$WORLD" -carriers 12 -switches 2 -warp 240 -demand 30 -console 127.0.0.1:8080 -avs-interval 300s -allow-private-peers > /tmp/gate-skyd.log 2>&1 &
PID=$!
trap 'kill $PID 2>/dev/null || true' EXIT
for i in $(seq 1 60); do
  if curl -sf -o /dev/null http://127.0.0.1:8080/stats/data.json; then break; fi
  sleep 2
done
sleep "$SECS"
/tmp/skycheck http://127.0.0.1:8080
# Worlds join worlds: a mirror of the same world -- every carrier renamed --
# boots beside it, told where it is, and the first world's lobby lists the
# mirror's carriers under the mirror's name.
go build -o /tmp/worldc ./cmd/worldc
/tmp/worldc -data data -countries "Portugal,Ireland" -carriers 4 -mirror Q -o /tmp/gate-mirror.json > /dev/null
/tmp/skyd -world /tmp/gate-mirror.json -carriers 4 -warp 240 -demand 5 -console 127.0.0.1:8083 -avs-interval 300s \
  -world-name mirror -world-code 1M -world-city AMS -public-url http://127.0.0.1:8083 -peer-world http://127.0.0.1:8080 -allow-private-peers > /tmp/gate-mirror.log 2>&1 &
MPID=$!
trap 'kill $PID $MPID 2>/dev/null || true' EXIT
JOINED=0
for i in $(seq 1 45); do
  if curl -sf http://127.0.0.1:8080/carriers.json | grep -q '"world":"mirror"'; then JOINED=1; break; fi
  sleep 2
done
if [ "$JOINED" != 1 ]; then echo 'the mirror world never joined'; tail -20 /tmp/gate-mirror.log; exit 1; fi
echo 'mirror world joined'
curl -sf http://127.0.0.1:8080/stats/data.json > /tmp/gate-stats.json
python3 - <<'PY'
import json
d=json.load(open('/tmp/gate-stats.json'))
t=d['totals']
print('movements', t['movements'], 'bookings', t['bookings'], 'undeliverable', t['undeliverable'], 'links', d['links'])
assert t['movements'] > 0, 'nothing flew'
assert t['bookings'] > 0, 'nothing sold'
assert t['undeliverable'] <= t['bookings'] // 10, 'more than a tenth of bookings undeliverable'
PY
grep -c 'level=ERROR' /tmp/gate-skyd.log || true
