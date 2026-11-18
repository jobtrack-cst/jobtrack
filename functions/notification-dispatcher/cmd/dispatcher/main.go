package main

import (
	"context"
	"log"
	"os"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/jobtrack-cst/jobtrack/functions/notification-dispatcher/internal"
)

func main() {
	required := []string{
		"SERVICE_NAME",
		"AUTH_ADDR",
		"NOTIFICATION_INBOX_ADDR",
		"INTERNAL_TOKEN",
		"EMAIL_SENDER_URL",
	}
	for _, key := range required {
		if os.Getenv(key) == "" {
			log.Fatalf("missing required environment variable: %s", key)
		}
	}

	log.Printf("Starting notification-dispatcher...")
	handler := internal.NewHandler()

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	if err := c.StartReceiver(context.Background(), handler.Handle); err != nil {
		log.Fatalf("failed to start receiver: %v", err)
	}
}
