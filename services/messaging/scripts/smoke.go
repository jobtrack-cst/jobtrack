//go:build ignore

// Run inside the Compose messaging container; credentials stay in its environment.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	messagingv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/messaging/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "JT-75 smoke failed:", err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	broker := os.Getenv("RABBITMQ_URL")
	pub, err := events.NewPublisher(broker)
	if err != nil {
		return err
	}
	defer pub.Close()
	conn, err := amqp.Dial(broker)
	if err != nil {
		return fmt.Errorf("connect observation client failed")
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return err
	}
	if err = ch.QueueBind(q.Name, "message.sent", "jobtrack.events", false, nil); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	if err != nil {
		return err
	}
	application, applicant, employer := uuid.NewString(), uuid.NewString(), uuid.NewString()
	data, _ := anypb.New(&eventsv1.ApplicationSubmitted{ApplicationId: application, ApplicantId: applicant, EmployerId: employer, JobId: uuid.NewString(), ResumeId: uuid.NewString()})
	envelope := &eventsv1.EventEnvelope{EventId: uuid.NewString(), Type: "application.submitted", Producer: "application", OccurredAt: timestamppb.Now(), Data: data}
	if err = pub.Publish(ctx, envelope); err != nil {
		return err
	}
	if err = pub.Publish(ctx, envelope); err != nil {
		return err
	}
	rpc, err := grpc.NewClient("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()))
	if err != nil {
		return err
	}
	defer rpc.Close()
	client := messagingv1.NewMessagingServiceClient(rpc)
	caller := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", applicant, "x-user-role", "applicant"))
	var response *messagingv1.SendMessageResponse
	for {
		response, err = client.SendMessage(caller, &messagingv1.SendMessageRequest{ApplicationId: application, Content: "JT-75 Compose smoke"})
		if err == nil {
			break
		}
		if status.Code(err) != codes.NotFound {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d := <-deliveries:
			e, err := events.UnmarshalEnvelope(d.Body)
			if err != nil {
				return err
			}
			var sent eventsv1.MessageSent
			if err = e.Data.UnmarshalTo(&sent); err != nil {
				return err
			}
			if sent.ApplicationId != application {
				continue
			}
			if sent.MessageId != response.GetMessage().GetMessageId() || sent.RecipientId != employer || e.Producer != "messaging" {
				return fmt.Errorf("outgoing payload mismatch")
			}
			fmt.Println("PASS: duplicate application.submitted -> trusted participants -> SendMessage -> message.sent via outbox")
			return nil
		}
	}
}
