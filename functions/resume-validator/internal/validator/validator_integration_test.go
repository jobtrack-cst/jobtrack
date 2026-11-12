//go:build integration

package validator_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/validator"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/jobtrack-cst/jobtrack/pkg/forwarder"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/anypb"
	"os"
)

type resumeServer struct {
	resumev1.UnimplementedResumeInternalServiceServer
	files map[string][]byte
}

func (s resumeServer) GetResumeFile(_ context.Context, req *resumev1.GetResumeFileRequest) (*resumev1.GetResumeFileResponse, error) {
	return &resumev1.GetResumeFileResponse{ResumeId: req.ResumeId, ApplicantId: "22222222-2222-4222-8222-222222222222", ContentType: "application/pdf", Content: s.files[req.ResumeId]}, nil
}

func TestValidator_IntegrationBrokerForwarderInternalRPCAndOutcome(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "rabbitmq:4.1-alpine", ExposedPorts: []string{"5672/tcp"}, Env: map[string]string{"RABBITMQ_DEFAULT_USER": "validator_test", "RABBITMQ_DEFAULT_PASS": "synthetic-password"}, WaitingFor: wait.ForListeningPort("5672/tcp")}, Started: true})
	if container != nil {
		t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(container)) })
	}
	require.NoError(t, err)
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5672/tcp")
	require.NoError(t, err)
	broker := (&url.URL{Scheme: "amqp", Host: net.JoinHostPort(host, port.Port()), User: url.UserPassword("validator_test", "synthetic-password"), Path: "/"}).String()
	conn, err := amqp.Dial(broker)
	require.NoError(t, err)
	defer conn.Close()
	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()
	require.NoError(t, forwarder.ResumeValidatorTopology().Declare(ch))
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	require.NoError(t, err)
	require.NoError(t, ch.QueueBind(q.Name, "resume.validated", "jobtrack.events", false, nil))
	require.NoError(t, ch.QueueBind(q.Name, "resume.rejected", "jobtrack.events", false, nil))
	results, err := ch.ConsumeWithContext(ctx, q.Name, "", true, true, false, false, nil)
	require.NoError(t, err)
	listener := bufconn.Listen(4 * 1024 * 1024)
	defer listener.Close()
	auth, err := authctx.InternalUnaryServerInterceptor("synthetic-token")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(auth))
	cases := []struct {
		name    string
		content []byte
		kind    string
	}{
		{"valid direct catalog", pdf(0), "resume.validated"},
		{"literal string spoof", pdfWithCatalog(`<< /Bogus (/Type /Catalog) /Pages 2 0 R >>`), "resume.rejected"},
		{"comment spoof", pdfWithCatalog("<< % /Type /Catalog\n/Pages 2 0 R >>"), "resume.rejected"},
		{"nested dictionary spoof", pdfWithCatalog(`<< /Bogus << /Type /Catalog >> /Pages 2 0 R >>`), "resume.rejected"},
	}
	files := make(map[string][]byte)
	for i, tc := range cases {
		files[fmt.Sprintf("11111111-1111-4111-8111-%012d", i+1)] = tc.content
	}
	resumev1.RegisterResumeInternalServiceServer(server, resumeServer{files: files})
	go server.Serve(listener)
	defer server.Stop()
	clientAuth, err := grpcclient.InternalUnaryClientInterceptor("synthetic-token", "resume-validator")
	require.NoError(t, err)
	client, err := grpc.NewClient("passthrough:///resume", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithChainUnaryInterceptor(clientAuth))
	require.NoError(t, err)
	defer client.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	httpServer := httptest.NewServer(validator.Handler{Files: resumev1.NewResumeInternalServiceClient(client), Events: validator.BrokerPublisher{URL: broker}, Logger: logger})
	defer httpServer.Close()
	done := make(chan error, 1)
	go func() {
		done <- forwarder.Run(ctx, forwarder.Config{Broker: broker, Sink: httpServer.URL, Function: "resume-validator", Bindings: []string{"resume.uploaded"}}, logger)
	}()
	defer func() { cancel(); <-done }()
	fixture, err := os.ReadFile("../../../../tests/contract/fixtures/resume.uploaded.json")
	require.NoError(t, err)
	source, err := events.UnmarshalEnvelope(fixture)
	require.NoError(t, err)
	publisher, err := events.NewPublisher(broker)
	require.NoError(t, err)
	defer publisher.Close()

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resumeID := fmt.Sprintf("11111111-1111-4111-8111-%012d", i+1)
			source.EventId = fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
			source.Data, err = anypb.New(&eventsv1.ResumeUploaded{ResumeId: resumeID, ApplicantId: "22222222-2222-4222-8222-222222222222"})
			require.NoError(t, err)
			require.NoError(t, publisher.Publish(ctx, source))
			select {
			case delivery := <-results:
				result, err := events.UnmarshalEnvelope(delivery.Body)
				require.NoError(t, err)
				require.Equal(t, tc.kind, result.Type)
				require.Equal(t, "resume-validator", result.Producer)
				if tc.kind == "resume.rejected" {
					rejected := new(eventsv1.ResumeRejected)
					require.NoError(t, result.Data.UnmarshalTo(rejected))
					require.Equal(t, resumeID, rejected.ResumeId)
					require.Equal(t, "invalid PDF catalog signature", rejected.Reason)
				}
			case <-ctx.Done():
				t.Fatal("no real validator result received")
			}
		})
	}
}
