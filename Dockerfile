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
RUN /out/worldc -data data \
      -countries "United Kingdom,France,Germany,Netherlands,Spain,Italy" \
      -o /out/europe.json

FROM alpine:3.20
RUN addgroup -S sky && adduser -S sky -G sky
COPY --from=build /out/skyd /usr/local/bin/skyd
COPY --from=build /out/europe.json /etc/wholesky/europe.json
USER sky
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/skyd"]
CMD ["-world", "/etc/wholesky/europe.json", \
     "-carriers", "20", "-demand", "20", "-warp", "120", \
     "-console", "0.0.0.0:8080"]
