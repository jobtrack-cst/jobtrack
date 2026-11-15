# Build from the repository root: docker build -f functions/deadline-reminder/Dockerfile .
FROM golang:1.27.3-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download
COPY gen/go/ ./gen/go/
COPY pkg/ ./pkg/
COPY functions/deadline-reminder/ ./functions/deadline-reminder/
RUN go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/deadline-reminder ./functions/deadline-reminder/cmd/server

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/deadline-reminder /usr/local/bin/deadline-reminder

ENV SERVICE_NAME=deadline-reminder HTTP_PORT=8080
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/deadline-reminder"]
