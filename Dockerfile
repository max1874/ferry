# The build stage runs on the builder's own platform and cross-compiles, so a
# multi-architecture image does not compile Go under emulation.
FROM --platform=$BUILDPLATFORM golang:1.26.3-alpine AS build

ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/ferry ./cmd/ferry

FROM alpine:3.23

LABEL org.opencontainers.image.source="https://github.com/max1874/ferry" \
      org.opencontainers.image.licenses="Apache-2.0"

RUN addgroup -S ferry \
    && adduser -S -D -H -u 10001 -G ferry ferry \
    && mkdir -p /data \
    && chown ferry:ferry /data

COPY --from=build /out/ferry /usr/local/bin/ferry
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

USER ferry
VOLUME ["/data"]
EXPOSE 42817
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
