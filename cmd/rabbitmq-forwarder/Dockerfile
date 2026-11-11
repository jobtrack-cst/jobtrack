# Build from repository root.
FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY gen/go ./gen/go
COPY pkg/events ./pkg/events
COPY pkg/logging ./pkg/logging
COPY pkg/forwarder ./pkg/forwarder
COPY cmd/rabbitmq-forwarder ./cmd/rabbitmq-forwarder
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -o /forwarder ./cmd/rabbitmq-forwarder
FROM scratch
COPY --from=build /forwarder /forwarder
USER 10001:10001
ENTRYPOINT ["/forwarder"]
