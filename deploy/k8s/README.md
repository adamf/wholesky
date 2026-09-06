# Running a world on Kubernetes

The same image the Fly demo runs, laid out as the demo is: one core (the
switches, the instruments, the lobby, the state), three distribution
systems, two regions of carrier tenants, and an Envoy edge in front. The
manifests are Kustomize: a `base/` that is the world, and overlays for a
GKE Standard cluster and for a laptop.

```
deploy/k8s/
  base/            the world: core, gds, region StatefulSets; the Envoy edge; policies
  overlays/gke/    static address, Let's Encrypt via cert-manager, GKE storage class
  overlays/kind/   NodePorts, a self-signed certificate, one small world
```

## What Envoy does here

Envoy terminates TLS for the console and the API and redirects plain
HTTP, with stream and route timeouts off so the event streams stay open
for hours. The two switch ports, 7000 and 7001, pass through as raw TCP:
a Type B link is a long-lived plaintext socket and reaches the core
untouched, and the world's own listener reaps a quiet one (jetway's
`idle_timeout`), not the edge. The Service is a layer-4 load balancer with
`externalTrafficPolicy: Local`, so the world sees the visitor's address
and its per-address limits work. This is the shape the Fly deployment
could not have: Fly's shared IPv4 carries only HTTP, which is why the
demo's trunk to the mirror world never came up there.

## GKE

Once, for the project:

```sh
gcloud container clusters create-auto wholesky --region europe-west2   # or a Standard cluster with an e2-standard-8 pool
gcloud compute addresses create wholesky-edge --region europe-west2
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
```

Then edit `overlays/gke/`: the address into `edge-static-ip.yaml`, the
host name into the `PUBLIC_HOST` literal and `certificate.yaml`, your
project into the ClusterIssuer (the DNS-01 solver needs a service account
with `roles/dns.admin` on the zone, bound to cert-manager's service
account by Workload Identity; the cert-manager documentation has the
four commands). Put an A and an AAAA record for the host at the address.
Make the link secret, which every machine of the world shares and which
keys every token the world hands out:

```sh
cp overlays/gke/link.env.example overlays/gke/link.env
sed -i "s/replace-me.*/$(openssl rand -hex 24)/" overlays/gke/link.env   # kept out of git
kubectl apply -k overlays/gke
kubectl -n wholesky rollout status statefulset/core
```

The world takes a minute to stand its five machines up; then
`https://<host>/ops/` is the lobby and `<host>:7000` is the first switch,
which a player's own jetway node dials with the token from its start
pack. Another world joins with `-peer-world https://<host>` and its trunk
comes up on 7000 -- over TCP that is actually TCP.

Two `e2-standard-8` nodes carry the demo's six processes (about
$400 a month on demand, $250 with a one-year commitment); the address and
the load balancer add about $25. A third of the Fly bill for the same
world, without the shared-address problem.

## A laptop

```sh
docker build -t ghcr.io/adamf/wholesky:dev . && kind load docker-image ghcr.io/adamf/wholesky:dev
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
kubectl apply -k overlays/kind
```

The lobby is at `https://localhost:30443/ops/` (a self-signed
certificate) and the switch at `localhost:30700`. The overlay runs one
region and one distribution system at a fraction of the memory: the
mirror world's size.

## Operating it

- **Upgrade:** `kustomize edit set image ghcr.io/adamf/wholesky=ghcr.io/adamf/wholesky:v0.2.0`
  in the overlay, then `kubectl apply -k`. The image workflow
  (`.github/workflows/image.yml`) publishes `latest`, the commit and every
  tag to GitHub's registry.
- **State:** seats, claims, joined worlds and finished runs live on the
  core's volume at `/data`; the regions keep theirs at the core over the
  control plane, behind the link secret. A redeploy keeps them.
- **Re-sharding:** the region StatefulSet's replica count is the shard
  count. Change both `REGION_SHARDS` and `replicas` together and restart
  the set; the carriers redistribute.
- **The operator's key:** the link secret is also what the sky's clock and
  the fleet's circuit controls ask for, once, in the browser.
- **Profiles:** `kubectl -n wholesky port-forward core-0 6060` then
  `go tool pprof http://localhost:6060/debug/pprof/heap`; pprof listens on
  loopback only.
- **What is not here:** a Postgres for the carriers' books (`-tenant-dsn`),
  which a world that must survive a restart with its bookings would add
  (jetway's `docs/production-gcp.md` has the topology); metrics scraping
  (`/metrics` on every node is Prometheus text; add a PodMonitor if the
  operator is installed); and a second core, which would be a second
  world.
