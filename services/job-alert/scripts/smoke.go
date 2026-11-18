//go:build ignore

// Run inside the Compose job-alert container; credentials stay in its environment.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	alertv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/job_alert/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "JT-76 smoke failed:", err)
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
	if err = ch.QueueBind(q.Name, "job_alert.matched", "jobtrack.events", false, nil); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	if err != nil {
		return err
	}
	applicant, jobID := uuid.NewString(), uuid.NewString()
	rpc, err := grpc.NewClient("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()))
	if err != nil {
		return err
	}
	defer rpc.Close()
	client := alertv1.NewJobAlertServiceClient(rpc)
	caller := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", applicant, "x-user-role", "applicant"))
	category, tag := "10000000-0000-4000-8000-000000000001", "20000000-0000-4000-8000-000000000001"
	response, err := client.CreateJobAlert(caller, &alertv1.CreateJobAlertRequest{CategoryIds: []string{category}, TagIds: []string{tag}, Location: "Thimphu"})
	if err != nil {
		return fmt.Errorf("create subscription failed: %w", err)
	}
	defer client.DeleteJobAlert(caller, &alertv1.DeleteJobAlertRequest{AlertId: response.JobAlert.AlertId})
	data, _ := anypb.New(&eventsv1.JobPublished{JobId: jobID, EmployerId: uuid.NewString(), CategoryId: category, TagIds: []string{tag}, Location: "Thimphu", Title: "JT-76 synthetic matching job", Deadline: timestamppb.New(time.Now().Add(24 * time.Hour))})
	envelope := &eventsv1.EventEnvelope{EventId: uuid.NewString(), Type: "job.published", Producer: "job-posting", OccurredAt: timestamppb.Now(), Data: data}
	if err = pub.Publish(ctx, envelope); err != nil {
		return err
	}
	if err = pub.Publish(ctx, envelope); err != nil {
		return err
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
			var sent eventsv1.JobAlertMatched
			if err = e.Data.UnmarshalTo(&sent); err != nil {
				return err
			}
			if sent.JobId != jobID {
				continue
			}
			if sent.ApplicantId != applicant || e.Producer != "job-alert" {
				return fmt.Errorf("outgoing payload mismatch")
			}
			fmt.Println("PASS: real ResolveNames -> subscription -> duplicate job.published -> job_alert.matched via outbox")
			return nil
		}
	}
}
