# Build from the repository root: docker build -f services/company-verification/Dockerfile .
FROM golang:1.27.3-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download
COPY gen/go/ ./gen/go/
COPY pkg/ ./pkg/
COPY services/company-verification/ ./services/company-verification/
RUN go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/company-verification ./services/company-verification/cmd/server

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates
# Match the repository-relative migration path used by startup.
WORKDIR /app
COPY --from=builder /out/company-verification /usr/local/bin/company-verification
COPY services/company-verification/migrations/ ./services/company-verification/migrations/

ENV SERVICE_NAME=company-verification GRPC_PORT=9090 HTTP_PORT=8080
USER 10001:10001
EXPOSE 9090 8080
ENTRYPOINT ["/usr/local/bin/company-verification"]
