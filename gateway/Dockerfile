# Build from the repository root: docker build -f gateway/Dockerfile .
FROM golang:1.27.3-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download
COPY gen/go/ ./gen/go/
COPY pkg/ ./pkg/
COPY gateway/ ./gateway/
RUN go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/gateway ./gateway/cmd/server

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/gateway /usr/local/bin/gateway

# JWT_SECRET and the <SERVICE>_ADDR variables are supplied at run time, never baked in.
ENV SERVICE_NAME=gateway HTTP_PORT=8080
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]
