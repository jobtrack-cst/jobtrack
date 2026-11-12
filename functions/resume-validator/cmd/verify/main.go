// Command verify exercises real upload, broker delivery and final resume status.
// Run on the deployment network with RESUME_ADDR and RABBITMQ_URL configured.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/testpdf"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(os.Getenv("RESUME_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(grpcclient.UnaryClientInterceptor()))
	if err != nil {
		return err
	}
	defer conn.Close()
	client := resumev1.NewResumeServiceClient(conn)
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", uuid.NewString(), "x-user-role", "applicant"))
	broker, err := amqp.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		return fmt.Errorf("connect evidence broker failed")
	}
	defer broker.Close()
	ch, err := broker.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return err
	}
	if err = ch.QueueBind(q.Name, "resume.*", "jobtrack.events", false, nil); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	if err != nil {
		return err
	}
	pdf := testpdf.Document(0)
	for _, tc := range []struct {
		name string
		data []byte
		want resumev1.ResumeValidationStatus
	}{{"valid", pdf, resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_VALIDATED}, {"spoof", []byte("%PDF-1.7\nthis is plain text pretending to be PDF"), resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_REJECTED}, {"bare-xref", testpdf.BareXref(), resumev1.ResumeValidationStatus_RESUME_VALIDATION_STATUS_REJECTED}} {
		upload, err := client.UploadResume(ctx, &resumev1.UploadResumeRequest{FileName: "jt62-" + tc.name + ".pdf", ContentType: "application/pdf", Content: tc.data})
		if err != nil {
			return fmt.Errorf("upload %s: %w", tc.name, err)
		}
		id := upload.Resume.ResumeId
		var inputID, resultID string
		for inputID == "" || resultID == "" {
			select {
			case d := <-deliveries:
				e, err := events.UnmarshalEnvelope(d.Body)
				if err != nil {
					return err
				}
				switch e.Type {
				case "resume.uploaded":
					data := new(eventsv1.ResumeUploaded)
					if e.Data.UnmarshalTo(data) == nil && data.ResumeId == id {
						inputID = e.EventId
					}
				case "resume.validated":
					data := new(eventsv1.ResumeValidated)
					if e.Data.UnmarshalTo(data) == nil && data.ResumeId == id {
						resultID = e.EventId
					}
				case "resume.rejected":
					data := new(eventsv1.ResumeRejected)
					if e.Data.UnmarshalTo(data) == nil && data.ResumeId == id {
						resultID = e.EventId
					}
				}
			case <-ctx.Done():
				return fmt.Errorf("timed out observing broker events for %s", id)
			}
		}
		for {
			result, err := client.GetResume(ctx, &resumev1.GetResumeRequest{ResumeId: id})
			if err != nil {
				return err
			}
			if result.Resume.ValidationStatus == tc.want {
				json.NewEncoder(os.Stdout).Encode(map[string]string{"case": tc.name, "resume_id": id, "event_id": inputID, "result_event_id": resultID, "status": tc.want.String()})
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("final status timeout for %s", id)
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
	_, err = client.UploadResume(ctx, &resumev1.UploadResumeRequest{FileName: "jt62-non-pdf.txt", ContentType: "text/plain", Content: []byte("not a PDF")})
	if status.Code(err).String() != "InvalidArgument" {
		return fmt.Errorf("expected non-PDF upload InvalidArgument, got %s", status.Code(err))
	}
	fmt.Println("non-PDF upload: InvalidArgument (contract blocks storage/event; no REJECTED transition possible)")
	return nil
}
