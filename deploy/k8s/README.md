# Running a world on Kubernetes

These manifests run the same image as the Fly demo, in the demo's layout.
The layout is one core, three distribution systems, two regions of carrier
tenants and an Envoy edge in front. The core runs the switches, the
instruments, the lobby and the state. The manifests are Kustomize. The
`base/` directory is the world. The overlays are for a Google Kubernetes
Engine (GKE) Standard cluster and for a laptop.

```
deploy/k8s/
  base/            the world: core, gds, region StatefulSets; the Envoy edge; policies
  overlays/gke/    static address, Let's Encrypt via cert-manager, GKE storage class
  overlays/kind/   NodePorts, a self-signed certificate, one small world
```

## The Envoy edge

Envoy terminates TLS for the console and the API, and redirects plain
HTTP. Stream and route timeouts are off, and the event streams stay open
for hours. The two switch ports, 7000 and 7001, pass through as raw TCP. A
Type B link is a long-lived plaintext socket, and it reaches the core
unchanged. The core closes an idle link, through jetway's `idle_timeout`.
Envoy does not. The Service is a layer-4 load balancer with
`externalTrafficPolicy: Local`. The world therefore sees the visitor's
address, and its per-address limits work. The Fly deployment cannot have
this shape. Fly's shared IPv4 carries only HTTP, and for that reason the
demo's trunk to the mirror world never came up there.

## GKE

Run these commands once for the project:

```sh
gcloud container clusters create-auto wholesky --region europe-west2   # or a Standard cluster with an e2-standard-8 pool
gcloud compute addresses create wholesky-edge --region europe-west2
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
```

Then edit `overlays/gke/`. Put the address into `edge-static-ip.yaml`.
Put the host name into the `PUBLIC_HOST` literal and into
`certificate.yaml`. Put your project into the ClusterIssuer. The DNS-01
solver needs a service account with `roles/dns.admin` on the zone, bound
to cert-manager's service account by Workload Identity. The cert-manager
documentation has the 4 commands. Put an A record and an AAAA record for
the host at the address. Make the link secret. Every machine of the world
shares the link secret, and it keys every token the world issues:

```sh
cp overlays/gke/link.env.example overlays/gke/link.env
sed -i "s/replace-me.*/$(openssl rand -hex 24)/" overlays/gke/link.env   # kept out of git
kubectl apply -k overlays/gke
kubectl -n wholesky rollout status statefulset/core
```

The world takes a minute to start its 5 machines. Then
`https://<host>/ops/` is the lobby, and `<host>:7000` is the first
switch. A player's own jetway node dials the first switch with the token
from its start pack. Another world joins with `-peer-world https://<host>`,
and its trunk comes up on 7000 over raw TCP.

The demo's 6 processes run on 2 `e2-standard-8` nodes. They cost about
$400 a month on demand, or $250 with a 1-year commitment. The address and
the load balancer add about $25. That is a third of the Fly bill for the
same world, and it has no shared-address problem.

## A laptop

```sh
docker build -t ghcr.io/adamf/wholesky:dev . && kind load docker-image ghcr.io/adamf/wholesky:dev
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
kubectl apply -k overlays/kind
```

The lobby is at `https://localhost:30443/ops/`, with a self-signed
certificate. The switch is at `localhost:30700`. The overlay runs 1
region and 1 distribution system at a fraction of the memory. This is the
mirror world's size.

## Operation

- **Upgrade:** run `kustomize edit set image ghcr.io/adamf/wholesky=ghcr.io/adamf/wholesky:v0.2.0`
  in the overlay, then `kubectl apply -k`. The image workflow in
  `.github/workflows/image.yml` publishes `latest`, the commit and every
  tag to GitHub's registry.
- **State:** seats, claims, joined worlds and finished runs are on the
  core's volume at `/data`. The regions keep their state at the core over
  the control plane, behind the link secret. A redeploy keeps the state.
- **Re-sharding:** the region StatefulSet's replica count is the shard
  count. Change `REGION_SHARDS` and `replicas` together, then restart the
  set. The carriers redistribute.
- **The operator's key:** the world's clock controls and the fleet's
  circuit controls also require the link secret, once, in the browser.
- **Profiles:** run `kubectl -n wholesky port-forward core-0 6060`, then
  `go tool pprof http://localhost:6060/debug/pprof/heap`. pprof listens on
  loopback only.
- **Not included:** a Postgres for the carriers' books, set with
  `-tenant-dsn`. A world that must keep its bookings across a restart
  needs one, and jetway's `docs/production-gcp.md` has the topology.
  Metrics scraping is not included. `/metrics` on every node serves
  Prometheus text, so add a PodMonitor if the operator is installed. A
  second core is not included, because a second core is a second world.
