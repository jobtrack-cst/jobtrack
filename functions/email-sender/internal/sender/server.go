package sender

import (
	"net/http"
	"time"
)

const (
	ReadTimeout = 10 * time.Second
	SMTPTimeout = 8 * time.Second
	// HTTP writes must allow both the body read and SMTP before response margin.
	WriteTimeout    = ReadTimeout + SMTPTimeout + 4*time.Second
	ShutdownTimeout = WriteTimeout
)

// NewServer keeps listener deadlines aligned with the sequential handler budgets.
func NewServer(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: ReadTimeout, WriteTimeout: WriteTimeout, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
}
