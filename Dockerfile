FROM golang:1.26.3-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ferry ./cmd/ferry

FROM alpine:3.23

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
