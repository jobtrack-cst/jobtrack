# Build from the repository root: docker build -f services/saved-jobs/Dockerfile .
FROM golang:1.27.3-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download
COPY gen/go/ ./gen/go/
COPY pkg/ ./pkg/
COPY services/saved-jobs/ ./services/saved-jobs/
RUN go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/saved-jobs ./services/saved-jobs/cmd/server

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates
# Match the repository-relative migration path used by startup.
WORKDIR /app
COPY --from=builder /out/saved-jobs /usr/local/bin/saved-jobs
COPY services/saved-jobs/migrations/ ./services/saved-jobs/migrations/

ENV SERVICE_NAME=saved-jobs GRPC_PORT=9090 HTTP_PORT=8080
USER 10001:10001
EXPOSE 9090 8080
ENTRYPOINT ["/usr/local/bin/saved-jobs"]
