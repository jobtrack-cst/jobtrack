# Build from the repository root: docker build -f functions/email-sender/Dockerfile .
FROM golang:1.27.3-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download
COPY gen/go/ ./gen/go/
COPY pkg/ ./pkg/
COPY functions/email-sender/ ./functions/email-sender/
RUN go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/email-sender ./functions/email-sender/cmd/server

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/email-sender /usr/local/bin/email-sender

ENV SERVICE_NAME=email-sender HTTP_PORT=8080
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/email-sender"]
