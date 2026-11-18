# Build from the repository root: docker build -f functions/notification-dispatcher/Dockerfile .
FROM golang:1.27.3-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download
COPY gen/go/ ./gen/go/
COPY pkg/ ./pkg/
COPY functions/notification-dispatcher/ ./functions/notification-dispatcher/
RUN go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/notification-dispatcher ./functions/notification-dispatcher/cmd/dispatcher

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/notification-dispatcher /usr/local/bin/notification-dispatcher

ENV SERVICE_NAME=notification-dispatcher HTTP_PORT=8080
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/notification-dispatcher"]
