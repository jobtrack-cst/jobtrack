package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	inboxv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/notification_inbox/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/grpcclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Handler struct {
	authClient  authv1.AuthInternalServiceClient
	inboxClient inboxv1.NotificationInboxInternalServiceClient
	httpClient  *http.Client
	emailURL    string

	mu      sync.Mutex
	seenIDs map[string]time.Time
}

func NewHandler() *Handler {
	authAddr := os.Getenv("AUTH_ADDR")
	inboxAddr := os.Getenv("NOTIFICATION_INBOX_ADDR")
	token := os.Getenv("INTERNAL_TOKEN")
	serviceName := "notification-dispatcher"

	authInterceptor, err := grpcclient.InternalUnaryClientInterceptor(token, serviceName)
	if err != nil {
		panic(fmt.Sprintf("failed to create auth interceptor: %v", err))
	}
	inboxInterceptor, err := grpcclient.InternalUnaryClientInterceptor(token, serviceName)
	if err != nil {
		panic(fmt.Sprintf("failed to create inbox interceptor: %v", err))
	}

	authConn, err := grpc.NewClient(authAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(authInterceptor),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to auth: %v", err))
	}

	inboxConn, err := grpc.NewClient(inboxAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(inboxInterceptor),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to inbox: %v", err))
	}

	return &Handler{
		authClient:  authv1.NewAuthInternalServiceClient(authConn),
		inboxClient: inboxv1.NewNotificationInboxInternalServiceClient(inboxConn),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		emailURL:    os.Getenv("EMAIL_SENDER_URL"),
		seenIDs:     make(map[string]time.Time),
	}
}

func (h *Handler) Handle(ctx context.Context, event cloudevents.Event) error {
	h.mu.Lock()
	if _, exists := h.seenIDs[event.ID()]; exists {
		h.mu.Unlock()
		slog.Info("ignoring duplicate event", "event_id", event.ID())
		return nil
	}
	h.seenIDs[event.ID()] = time.Now()
	h.mu.Unlock()

	slog.Info("processing event", "event_type", event.Type(), "event_id", event.ID())

	var err error
	switch event.Type() {
	case "application.submitted":
		err = h.handleApplicationSubmitted(ctx, event)
	case "application.status_changed":
		err = h.handleApplicationStatusChanged(ctx, event)
	case "interview.scheduled":
		err = h.handleInterviewScheduled(ctx, event)
	case "offer.made":
		err = h.handleOfferMade(ctx, event)
	case "message.sent":
		err = h.handleMessageSent(ctx, event)
	case "job_alert.matched":
		err = h.handleJobAlertMatched(ctx, event)
	case "employer.verified":
		err = h.handleEmployerVerified(ctx, event)
	case "deadline.reminder":
		err = h.handleDeadlineReminder(ctx, event)
	default:
		slog.Warn("ignoring unknown event type", "event_type", event.Type())
		return nil
	}

	if err != nil {
		slog.Error("failed to handle event", "event_type", event.Type(), "error", err)
		return err
	}
	return nil
}

// --- Event Handlers ---

func (h *Handler) handleApplicationSubmitted(ctx context.Context, event cloudevents.Event) error {
	var data struct {
		ApplicationID string `json:"application_id"`
		ApplicantID   string `json:"applicant_id"`
		JobID         string `json:"job_id"`
		EmployerID    string `json:"employer_id"`
	}
	if err := unwrapEnvelope(event, &data); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	// Applicant in-app
	if err := h.sendInAppNotification(ctx, data.ApplicantID, "application_submitted", "Application Submitted", fmt.Sprintf("Your application for job %s has been submitted.", data.JobID), data.ApplicationID, event.ID()); err != nil {
		return err
	}
	// Employer in-app
	if err := h.sendInAppNotification(ctx, data.EmployerID, "application_received", "New Application Received", fmt.Sprintf("An applicant applied for your job %s.", data.JobID), data.ApplicationID, event.ID()); err != nil {
		return err
	}
	// Employer email
	if err := h.sendEmail(ctx, data.EmployerID, "New Application Received", fmt.Sprintf("You have a new application for job %s.", data.JobID)); err != nil {
		return err
	}
	return nil
}

func (h *Handler) handleApplicationStatusChanged(ctx context.Context, event cloudevents.Event) error {
	var data struct {
		ApplicationID string `json:"application_id"`
		ApplicantID   string `json:"applicant_id"`
		OldStatus     string `json:"old_status"`
		NewStatus     string `json:"new_status"`
	}
	if err := unwrapEnvelope(event, &data); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	title := fmt.Sprintf("Application Status: %s", data.NewStatus)
	body := fmt.Sprintf("Your application status changed from %s to %s.", data.OldStatus, data.NewStatus)

	if err := h.sendInAppNotification(ctx, data.ApplicantID, "application_status_changed", title, body, data.ApplicationID, event.ID()); err != nil {
		return err
	}
	if err := h.sendEmail(ctx, data.ApplicantID, title, body); err != nil {
		return err
	}
	return nil
}

func (h *Handler) handleInterviewScheduled(ctx context.Context, event cloudevents.Event) error {
	var data struct {
		InterviewID   string `json:"interview_id"`
		ApplicationID string `json:"application_id"`
		ApplicantID   string `json:"applicant_id"`
		ScheduledAt   string `json:"scheduled_at"`
	}
	if err := unwrapEnvelope(event, &data); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	title := "Interview Scheduled"
	body := fmt.Sprintf("Your interview is scheduled for %s.", data.ScheduledAt)

	if err := h.sendInAppNotification(ctx, data.ApplicantID, "interview_scheduled", title, body, data.ApplicationID, event.ID()); err != nil {
		return err
	}
	if err := h.sendEmail(ctx, data.ApplicantID, title, body); err != nil {
		return err
	}
	return nil
}

func (h *Handler) handleOfferMade(ctx context.Context, event cloudevents.Event) error {
	var data struct {
		OfferID       string `json:"offer_id"`
		ApplicationID string `json:"application_id"`
		ApplicantID   string `json:"applicant_id"`
	}
	if err := unwrapEnvelope(event, &data); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	title := "Job Offer Received"
	body := "Congratulations! You have received a job offer. Please review it."

	if err := h.sendInAppNotification(ctx, data.ApplicantID, "offer_made", title, body, data.ApplicationID, event.ID()); err != nil {
		return err
	}
	if err := h.sendEmail(ctx, data.ApplicantID, title, body); err != nil {
		return err
	}
	return nil
}

func (h *Handler) handleMessageSent(ctx context.Context, event cloudevents.Event) error {
	var data struct {
		ThreadID    string `json:"thread_id"`
		SenderID    string `json:"sender_id"`
		RecipientID string `json:"recipient_id"`
		MessageID   string `json:"message_id"`
	}
	if err := unwrapEnvelope(event, &data); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	title := "New Message"
	body := "You have received a new message."

	if err := h.sendInAppNotification(ctx, data.RecipientID, "message_sent", title, body, data.ThreadID, event.ID()); err != nil {
		return err
	}
	return nil
}

func (h *Handler) handleJobAlertMatched(ctx context.Context, event cloudevents.Event) error {
	var data struct {
		JobID       string `json:"job_id"`
		ApplicantID string `json:"applicant_id"`
	}
	if err := unwrapEnvelope(event, &data); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	title := "New Job Matches Your Alert"
	body := fmt.Sprintf("A new job (ID: %s) matches your alert criteria.", data.JobID)

	if err := h.sendInAppNotification(ctx, data.ApplicantID, "job_alert_matched", title, body, data.JobID, event.ID()); err != nil {
		return err
	}
	return nil
}

func (h *Handler) handleEmployerVerified(ctx context.Context, event cloudevents.Event) error {
	var data struct {
		EmployerID string `json:"employer_id"`
	}
	if err := unwrapEnvelope(event, &data); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	title := "Company Verified"
	body := "Your company has been successfully verified. You can now post jobs."

	if err := h.sendInAppNotification(ctx, data.EmployerID, "employer_verified", title, body, data.EmployerID, event.ID()); err != nil {
		return err
	}
	if err := h.sendEmail(ctx, data.EmployerID, title, body); err != nil {
		return err
	}
	return nil
}

func (h *Handler) handleDeadlineReminder(ctx context.Context, event cloudevents.Event) error {
	var data struct {
		JobID       string `json:"job_id"`
		ApplicantID string `json:"applicant_id"`
		Deadline    string `json:"deadline"`
	}
	if err := unwrapEnvelope(event, &data); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	title := "Job Deadline Approaching"
	body := fmt.Sprintf("The job you saved is closing on %s.", data.Deadline)

	if err := h.sendInAppNotification(ctx, data.ApplicantID, "deadline_reminder", title, body, data.JobID, event.ID()); err != nil {
		return err
	}
	if err := h.sendEmail(ctx, data.ApplicantID, title, body); err != nil {
		return err
	}
	return nil
}

// unwrapEnvelope extracts the nested "data" field from the raw event body.
// The forwarder sends the full domain envelope as the HTTP body.
func unwrapEnvelope(event cloudevents.Event, target interface{}) error {
	// First, unmarshal the raw body into an envelope wrapper
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := event.DataAs(&envelope); err != nil {
		return fmt.Errorf("failed to parse envelope: %w", err)
	}
	// Then unmarshal the inner data into the target struct
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("failed to parse event data: %w", err)
	}
	return nil
}

// --- Helpers ---

func (h *Handler) sendInAppNotification(ctx context.Context, recipientID, notifType, title, body, relatedID, sourceEventID string) error {
	_, err := h.inboxClient.CreateNotification(ctx, &inboxv1.CreateNotificationRequest{
		RecipientId:   recipientID,
		Type:          notifType,
		Title:         title,
		Body:          body,
		RelatedId:     relatedID,
		SourceEventId: sourceEventID,
	})
	return err
}

func (h *Handler) sendEmail(ctx context.Context, recipientID, subject, body string) error {
	userResp, err := h.authClient.GetUser(ctx, &authv1.GetUserRequest{UserId: recipientID})
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	payload := map[string]string{
		"recipient": userResp.User.Email,
		"subject":   subject,
		"body":      body,
	}
	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", h.emailURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("email-sender status %d", resp.StatusCode)
	}
	return nil
}
