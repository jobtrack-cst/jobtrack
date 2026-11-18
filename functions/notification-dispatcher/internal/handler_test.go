package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	inboxv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/notification_inbox/v1"
	"google.golang.org/grpc"
)

// --- Manual Mocks ---

type mockAuthClient struct {
	authv1.AuthInternalServiceClient
	GetUserFunc func(ctx context.Context, req *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error)
}

func (m *mockAuthClient) GetUser(ctx context.Context, req *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
	if m.GetUserFunc != nil {
		return m.GetUserFunc(ctx, req, opts...)
	}
	return &authv1.GetUserResponse{User: &authv1.User{Email: "test@example.com"}}, nil
}

type mockInboxClient struct {
	inboxv1.NotificationInboxInternalServiceClient
	CreateNotificationFunc func(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error)
}

func (m *mockInboxClient) CreateNotification(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error) {
	if m.CreateNotificationFunc != nil {
		return m.CreateNotificationFunc(ctx, req, opts...)
	}
	return &inboxv1.CreateNotificationResponse{}, nil
}

// --- Tests ---

func TestHandle_DeadlineReminder(t *testing.T) {
	// 1. Load the contract fixture (this is the data payload, not the full CloudEvent)
	fixtureData, err := os.ReadFile("../../../tests/contract/fixtures/deadline.reminder.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	// 2. Build a proper CloudEvent
	event := cloudevents.NewEvent()
	event.SetID("c9f7ca12-5db2-5d37-b133-1349f4c486dc")
	event.SetType("deadline.reminder")
	event.SetSource("deadline-reminder")
	event.SetTime(time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC))

	// 3. The fixture is the envelope: {"event_id": "...", "data": {...}}
	//    Parse it to extract just the inner "data" object.
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixtureData, &envelope); err != nil {
		t.Fatalf("failed to parse fixture envelope: %v", err)
	}
	if err := event.SetData(cloudevents.ApplicationJSON, envelope.Data); err != nil {
		t.Fatalf("failed to set event data: %v", err)
	}

	// 4. Set up the mock Email server
	emailCalled := false
	emailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailCalled = true
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer emailServer.Close()

	// 5. Set up the Handler with mocks
	inboxCalled := false
	mockInbox := &mockInboxClient{
		CreateNotificationFunc: func(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error) {
			inboxCalled = true
			if req.RecipientId != "22222222-2222-4222-8222-222222222222" {
				t.Errorf("wrong recipient: %s", req.RecipientId)
			}
			return &inboxv1.CreateNotificationResponse{}, nil
		},
	}

	mockAuth := &mockAuthClient{
		GetUserFunc: func(ctx context.Context, req *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
			return &authv1.GetUserResponse{User: &authv1.User{Email: "applicant@example.com"}}, nil
		},
	}

	handler := &Handler{
		authClient:  mockAuth,
		inboxClient: mockInbox,
		httpClient:  &http.Client{},
		emailURL:    emailServer.URL,
		seenIDs:     make(map[string]time.Time),
	}

	// 6. Handle the event
	ctx := context.Background()
	if err := handler.Handle(ctx, event); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// 7. Assertions
	if !inboxCalled {
		t.Error("expected CreateNotification to be called")
	}
	if !emailCalled {
		t.Error("expected email-sender to be called")
	}
}

func TestHandle_ApplicationSubmitted(t *testing.T) {
	// 1. Load the contract fixture
	fixtureData, err := os.ReadFile("../../../tests/contract/fixtures/application.submitted.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	// 2. Build a proper CloudEvent
	event := cloudevents.NewEvent()
	event.SetID("66666666-6666-4666-8666-666666666665")
	event.SetType("application.submitted")
	event.SetSource("application")
	event.SetTime(time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC))

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixtureData, &envelope); err != nil {
		t.Fatalf("failed to parse fixture envelope: %v", err)
	}
	if err := event.SetData(cloudevents.ApplicationJSON, envelope.Data); err != nil {
		t.Fatalf("failed to set event data: %v", err)
	}

	// 3. Set up the mock Email server
	emailCalled := false
	emailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer emailServer.Close()

	// 4. Set up the Handler with mocks
	inboxCalls := 0
	mockInbox := &mockInboxClient{
		CreateNotificationFunc: func(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error) {
			inboxCalls++
			return &inboxv1.CreateNotificationResponse{}, nil
		},
	}

	mockAuth := &mockAuthClient{
		GetUserFunc: func(ctx context.Context, req *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
			return &authv1.GetUserResponse{User: &authv1.User{Email: "employer@example.com"}}, nil
		},
	}

	handler := &Handler{
		authClient:  mockAuth,
		inboxClient: mockInbox,
		httpClient:  &http.Client{},
		emailURL:    emailServer.URL,
		seenIDs:     make(map[string]time.Time),
	}

	// 5. Handle the event
	ctx := context.Background()
	if err := handler.Handle(ctx, event); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}

	// 6. Assertions
	// The applicant gets an in-app notification, the employer gets an in-app notification AND an email
	if inboxCalls != 2 {
		t.Errorf("expected 2 CreateNotification calls (applicant + employer), got %d", inboxCalls)
	}
	if !emailCalled {
		t.Error("expected email-sender to be called for the employer")
	}
}

func TestHandle_ApplicationStatusChanged(t *testing.T) {
	fixtureData, err := os.ReadFile("../../../tests/contract/fixtures/application.status_changed.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	event := cloudevents.NewEvent()
	event.SetID("77777777-7777-4777-8777-777777777777")
	event.SetType("application.status_changed")
	event.SetSource("application-status")
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixtureData, &envelope); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	if err := event.SetData(cloudevents.ApplicationJSON, envelope.Data); err != nil {
		t.Fatalf("failed to set data: %v", err)
	}

	emailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer emailServer.Close()

	inboxCalls := 0
	mockInbox := &mockInboxClient{
		CreateNotificationFunc: func(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error) {
			inboxCalls++
			return &inboxv1.CreateNotificationResponse{}, nil
		},
	}
	mockAuth := &mockAuthClient{
		GetUserFunc: func(ctx context.Context, req *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
			return &authv1.GetUserResponse{User: &authv1.User{Email: "applicant@example.com"}}, nil
		},
	}
	handler := &Handler{
		authClient: mockAuth, inboxClient: mockInbox,
		httpClient: &http.Client{}, emailURL: emailServer.URL,
		seenIDs: make(map[string]time.Time),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if inboxCalls != 1 {
		t.Errorf("expected 1 notification, got %d", inboxCalls)
	}
}

func TestHandle_InterviewScheduled(t *testing.T) {
	fixtureData, err := os.ReadFile("../../../tests/contract/fixtures/interview.scheduled.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	event := cloudevents.NewEvent()
	event.SetID("88888888-8888-4888-8888-888888888888")
	event.SetType("interview.scheduled")
	event.SetSource("interview")
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixtureData, &envelope); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	if err := event.SetData(cloudevents.ApplicationJSON, envelope.Data); err != nil {
		t.Fatalf("failed to set data: %v", err)
	}

	emailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer emailServer.Close()
	inboxCalls := 0
	mockInbox := &mockInboxClient{
		CreateNotificationFunc: func(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error) {
			inboxCalls++
			return &inboxv1.CreateNotificationResponse{}, nil
		},
	}
	mockAuth := &mockAuthClient{
		GetUserFunc: func(ctx context.Context, req *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
			return &authv1.GetUserResponse{User: &authv1.User{Email: "applicant@example.com"}}, nil
		},
	}
	handler := &Handler{
		authClient: mockAuth, inboxClient: mockInbox,
		httpClient: &http.Client{}, emailURL: emailServer.URL,
		seenIDs: make(map[string]time.Time),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if inboxCalls != 1 {
		t.Errorf("expected 1 notification, got %d", inboxCalls)
	}
}

func TestHandle_OfferMade(t *testing.T) {
	fixtureData, err := os.ReadFile("../../../tests/contract/fixtures/offer.made.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	event := cloudevents.NewEvent()
	event.SetID("99999999-9999-4999-8999-999999999999")
	event.SetType("offer.made")
	event.SetSource("offer")
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixtureData, &envelope); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	if err := event.SetData(cloudevents.ApplicationJSON, envelope.Data); err != nil {
		t.Fatalf("failed to set data: %v", err)
	}
	emailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer emailServer.Close()
	inboxCalls := 0
	mockInbox := &mockInboxClient{
		CreateNotificationFunc: func(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error) {
			inboxCalls++
			return &inboxv1.CreateNotificationResponse{}, nil
		},
	}
	mockAuth := &mockAuthClient{
		GetUserFunc: func(ctx context.Context, req *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
			return &authv1.GetUserResponse{User: &authv1.User{Email: "applicant@example.com"}}, nil
		},
	}
	handler := &Handler{
		authClient: mockAuth, inboxClient: mockInbox,
		httpClient: &http.Client{}, emailURL: emailServer.URL,
		seenIDs: make(map[string]time.Time),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if inboxCalls != 1 {
		t.Errorf("expected 1 notification, got %d", inboxCalls)
	}
}

func TestHandle_MessageSent(t *testing.T) {
	fixtureData, err := os.ReadFile("../../../tests/contract/fixtures/message.sent.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	event := cloudevents.NewEvent()
	event.SetID("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	event.SetType("message.sent")
	event.SetSource("messaging")
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixtureData, &envelope); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	if err := event.SetData(cloudevents.ApplicationJSON, envelope.Data); err != nil {
		t.Fatalf("failed to set data: %v", err)
	}
	inboxCalls := 0
	mockInbox := &mockInboxClient{
		CreateNotificationFunc: func(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error) {
			inboxCalls++
			return &inboxv1.CreateNotificationResponse{}, nil
		},
	}
	handler := &Handler{
		authClient: &mockAuthClient{}, inboxClient: mockInbox,
		httpClient: &http.Client{}, emailURL: "http://invalid",
		seenIDs: make(map[string]time.Time),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if inboxCalls != 1 {
		t.Errorf("expected 1 notification, got %d", inboxCalls)
	}
}

func TestHandle_JobAlertMatched(t *testing.T) {
	fixtureData, err := os.ReadFile("../../../tests/contract/fixtures/job_alert.matched.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	event := cloudevents.NewEvent()
	event.SetID("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	event.SetType("job_alert.matched")
	event.SetSource("job-alert")
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixtureData, &envelope); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	if err := event.SetData(cloudevents.ApplicationJSON, envelope.Data); err != nil {
		t.Fatalf("failed to set data: %v", err)
	}
	inboxCalls := 0
	mockInbox := &mockInboxClient{
		CreateNotificationFunc: func(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error) {
			inboxCalls++
			return &inboxv1.CreateNotificationResponse{}, nil
		},
	}
	handler := &Handler{
		authClient: &mockAuthClient{}, inboxClient: mockInbox,
		httpClient: &http.Client{}, emailURL: "http://invalid",
		seenIDs: make(map[string]time.Time),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if inboxCalls != 1 {
		t.Errorf("expected 1 notification, got %d", inboxCalls)
	}
}

func TestHandle_EmployerVerified(t *testing.T) {
	fixtureData, err := os.ReadFile("../../../tests/contract/fixtures/employer.verified.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	event := cloudevents.NewEvent()
	event.SetID("cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	event.SetType("employer.verified")
	event.SetSource("company-verification")
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(fixtureData, &envelope); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	if err := event.SetData(cloudevents.ApplicationJSON, envelope.Data); err != nil {
		t.Fatalf("failed to set data: %v", err)
	}
	emailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer emailServer.Close()
	inboxCalls := 0
	mockInbox := &mockInboxClient{
		CreateNotificationFunc: func(ctx context.Context, req *inboxv1.CreateNotificationRequest, opts ...grpc.CallOption) (*inboxv1.CreateNotificationResponse, error) {
			inboxCalls++
			return &inboxv1.CreateNotificationResponse{}, nil
		},
	}
	mockAuth := &mockAuthClient{
		GetUserFunc: func(ctx context.Context, req *authv1.GetUserRequest, opts ...grpc.CallOption) (*authv1.GetUserResponse, error) {
			return &authv1.GetUserResponse{User: &authv1.User{Email: "employer@example.com"}}, nil
		},
	}
	handler := &Handler{
		authClient: mockAuth, inboxClient: mockInbox,
		httpClient: &http.Client{}, emailURL: emailServer.URL,
		seenIDs: make(map[string]time.Time),
	}
	if err := handler.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if inboxCalls != 1 {
		t.Errorf("expected 1 notification, got %d", inboxCalls)
	}
}
