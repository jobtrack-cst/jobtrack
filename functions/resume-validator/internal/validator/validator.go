// Package validator handles stateless resume validation deliveries.
package validator

import (
	"context"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const MaxBytes = 2_000_000

// FileClient is the only approved synchronous dependency.
type FileClient interface {
	GetResumeFile(context.Context, *resumev1.GetResumeFileRequest, ...grpc.CallOption) (*resumev1.GetResumeFileResponse, error)
}
type Publisher interface {
	Publish(context.Context, *eventsv1.EventEnvelope) error
}

type Handler struct {
	Files  FileClient
	Events Publisher
	Logger *slog.Logger
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || r.Header.Get("Ce-Specversion") != "1.0" || r.Header.Get("Ce-Source") == "" {
		http.Error(w, "invalid CloudEvent", 400)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
	if err != nil {
		http.Error(w, "invalid event body", 400)
		return
	}
	envelope, err := events.UnmarshalEnvelope(body)
	if err != nil || envelope.GetType() != "resume.uploaded" || envelope.GetProducer() != "resume" || r.Header.Get("Ce-Id") != envelope.GetEventId() || r.Header.Get("Ce-Type") != envelope.GetType() {
		http.Error(w, "invalid resume event", 400)
		return
	}
	uploaded := new(eventsv1.ResumeUploaded)
	if envelope.Data.UnmarshalTo(uploaded) != nil || !validID(uploaded.ResumeId) || !validID(uploaded.ApplicantId) {
		http.Error(w, "invalid resume data", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	file, err := h.Files.GetResumeFile(ctx, &resumev1.GetResumeFileRequest{ResumeId: uploaded.ResumeId})
	if err != nil {
		h.Logger.Warn("resume fetch failed", "event_id", envelope.EventId)
		http.Error(w, "resume fetch failed", 503)
		return
	}
	if file.GetResumeId() != uploaded.ResumeId || file.GetApplicantId() != uploaded.ApplicantId {
		http.Error(w, "resume identity mismatch", 422)
		return
	}
	reason := Validate(file.ContentType, file.Content)
	kind := "resume.validated"
	var data proto.Message = &eventsv1.ResumeValidated{ResumeId: uploaded.ResumeId, ApplicantId: uploaded.ApplicantId}
	if reason != "" {
		kind = "resume.rejected"
		data = &eventsv1.ResumeRejected{ResumeId: uploaded.ResumeId, ApplicantId: uploaded.ApplicantId, Reason: reason}
	}
	packed, err := anypb.New(data)
	if err != nil {
		http.Error(w, "encode outcome failed", 500)
		return
	}
	// Deterministic result identity makes retries/restarts deduplicate in Resume's
	// transactional processed_events table without keeping function-local state.
	result := &eventsv1.EventEnvelope{EventId: uuid.NewSHA1(uuid.NameSpaceOID, []byte("resume-validator:"+envelope.EventId)).String(), Type: kind, OccurredAt: envelope.OccurredAt, Producer: "resume-validator", Data: packed}
	if err := h.Events.Publish(ctx, result); err != nil {
		h.Logger.Warn("outcome publish failed", "event_id", envelope.EventId)
		http.Error(w, "outcome publish failed", 503)
		return
	}
	h.Logger.Info("resume validation published", "event_id", envelope.EventId, "result_event_id", result.EventId, "resume_id", uploaded.ResumeId, "outcome", kind)
	w.WriteHeader(http.StatusNoContent)
}
func validID(s string) bool { _, err := uuid.Parse(s); return err == nil && len(s) == 36 }

func Required(value string) bool { return strings.TrimSpace(value) != "" }
