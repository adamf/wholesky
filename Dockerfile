# The public demo image: a European sky.
#
# The world is compiled at build time from the vendored OpenFlights snapshot,
# so the image carries a deterministic manifest rather than compiling on every
# boot. jetway is fetched by tag from the module proxy -- no side-by-side
# checkout, which is the reason wholesky pins a version instead of tracking
# head.

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/skyd   ./cmd/skyd \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worldc ./cmd/worldc
RUN /out/worldc -data data -o /out/whole.json

FROM alpine:3.20
RUN addgroup -S sky && adduser -S sky -G sky
COPY --from=build /out/skyd /usr/local/bin/skyd
COPY --from=build /out/whole.json /etc/wholesky/whole.json
USER sky
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/skyd"]
CMD ["-world", "/etc/wholesky/whole.json", \
     "-carriers", "0", "-demand", "60", "-warp", "60", \
     "-avs-interval", "60s", "-max-messages", "2000", "-max-records", "4000", \
     "-tenant-max-messages", "300", "-tenant-max-records", "800", \
     "-gds", "3", \
     "-stats-snapshot", "/tmp/wholesky-stats.json", \
     "-console", "0.0.0.0:8080"]
