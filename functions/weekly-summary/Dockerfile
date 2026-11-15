# Build from the repository root: docker build -f functions/weekly-summary/Dockerfile .
FROM golang:1.27.3-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download
COPY gen/go/ ./gen/go/
COPY pkg/ ./pkg/
COPY functions/weekly-summary/ ./functions/weekly-summary/
RUN go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/weekly-summary ./functions/weekly-summary/cmd/server

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/weekly-summary /usr/local/bin/weekly-summary

ENV SERVICE_NAME=weekly-summary HTTP_PORT=8080
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/weekly-summary"]
