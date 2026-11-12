package validator_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/testpdf"
	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/validator"
	eventsv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/events/v1"
	resumev1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/resume/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/events"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func pdf(size int) []byte { return testpdf.Document(size) }
func TestValidate_SignaturesAndBoundaries(t *testing.T) {
	for _, size := range []int{1_999_999, 2_000_000, 2_000_001} {
		require.Len(t, pdf(size), size)
	}
	for _, tc := range []struct {
		name, kind string
		data       []byte
		valid      bool
	}{
		{"valid", "application/pdf", pdf(0), true},
		{"below limit", "application/pdf", pdf(1_999_999), true},
		{"at limit", "application/pdf", pdf(2_000_000), true},
		{"over limit", "application/pdf", pdf(2_000_001), false},
		{"empty", "application/pdf", nil, false},
		{"one byte", "application/pdf", []byte("%"), false},
		{"wrong type", "text/plain", pdf(0), false},
		{"MIME spoof", "application/pdf", []byte("<html>fake</html>"), false},
		{"prefix spoof", "application/pdf", []byte("%PDF-1.7\nhello world"), false},
		{"bare xref review regression", "application/pdf", testpdf.BareXref(), false},
		{"invalid version", "application/pdf", bytes.Replace(pdf(0), []byte("1.4"), []byte("9.9"), 1), false},
		{"embedded signature", "application/pdf", append([]byte("MZ"), pdf(0)...), false},
		{"truncated", "application/pdf", pdf(0)[:100], false},
		{"xref object offset mismatch", "application/pdf", bytes.Replace(pdf(0), []byte("0000000009 00000 n"), []byte("0000000010 00000 n"), 1), false},
		{"xref count overflow", "application/pdf", bytes.Replace(pdf(0), []byte("xref\n0 6"), []byte("xref\n0 99999999999999999999"), 1), false},
		{"xref entry missing", "application/pdf", bytes.Replace(pdf(0), []byte("0000000009 00000 n \n"), nil, 1), false},
		{"trailer missing", "application/pdf", bytes.Replace(pdf(0), []byte("trailer"), []byte("garbage"), 1), false},
		{"trailer size mismatch", "application/pdf", bytes.Replace(pdf(0), []byte("/Size 6"), []byte("/Size 1"), 1), false},
		{"wrong root generation", "application/pdf", bytes.Replace(pdf(0), []byte("/Root 1 0 R"), []byte("/Root 1 1 R"), 1), false},
		{"root is not catalog", "application/pdf", bytes.Replace(pdf(0), []byte("/Root 1 0 R"), []byte("/Root 2 0 R"), 1), false},
		{"bad xref", "application/pdf", bytes.Replace(pdf(0), []byte("startxref\n"), []byte("startxref\n999"), 1), false},
		{"appended executable", "application/pdf", append(pdf(0), []byte("MZpayload")...), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason := validator.Validate(tc.kind, tc.data)
			require.Equal(t, tc.valid, reason == "", reason)
		})
	}
}

type files struct {
	value *resumev1.GetResumeFileResponse
	err   error
	calls int
}

func (f *files) GetResumeFile(context.Context, *resumev1.GetResumeFileRequest, ...grpc.CallOption) (*resumev1.GetResumeFileResponse, error) {
	f.calls++
	return f.value, f.err
}

type publisher struct {
	events []*eventsv1.EventEnvelope
	err    error
}

func (p *publisher) Publish(_ context.Context, e *eventsv1.EventEnvelope) error {
	p.events = append(p.events, e)
	return p.err
}
func TestHandler_ContractAndFailures(t *testing.T) {
	fixture, err := os.ReadFile("../../../../tests/contract/fixtures/resume.uploaded.json")
	require.NoError(t, err)
	source, err := events.UnmarshalEnvelope(fixture)
	require.NoError(t, err)
	for _, tc := range []struct {
		name                 string
		content              []byte
		fetchErr, publishErr error
		headerID             string
		applicantID          string
		code                 int
		kind                 string
	}{
		{name: "validated", content: pdf(0), code: 204, kind: "resume.validated"},
		{name: "rejected catalog string spoof", content: pdfWithCatalog(`<< /Bogus (/Type /Catalog) /Pages 2 0 R >>`), code: 204, kind: "resume.rejected"},
		{name: "rejected bare xref", content: testpdf.BareXref(), code: 204, kind: "resume.rejected"},
		{name: "rejected spoof", content: []byte("%PDF-fake"), code: 204, kind: "resume.rejected"},
		{name: "fetch retry", fetchErr: errors.New("unavailable"), code: 503},
		{name: "publish retry", content: pdf(0), publishErr: errors.New("unconfirmed"), code: 503, kind: "resume.validated"},
		{name: "header mismatch", headerID: "wrong", code: 400},
		{name: "file owner mismatch", applicantID: "33333333-3333-4333-8333-333333333333", content: pdf(0), code: 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &files{value: &resumev1.GetResumeFileResponse{ResumeId: "11111111-1111-4111-8111-111111111111", ApplicantId: "22222222-2222-4222-8222-222222222222", ContentType: "application/pdf", Content: tc.content}, err: tc.fetchErr}
			if tc.applicantID != "" {
				f.value.ApplicantId = tc.applicantID
			}
			p := &publisher{err: tc.publishErr}
			h := validator.Handler{Files: f, Events: p, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			call := func() *httptest.ResponseRecorder {
				req := httptest.NewRequest("POST", "/", bytes.NewReader(fixture))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Ce-Specversion", "1.0")
				req.Header.Set("Ce-Source", "/jobtrack/rabbitmq")
				id := source.EventId
				if tc.headerID != "" {
					id = tc.headerID
				}
				req.Header.Set("Ce-Id", id)
				req.Header.Set("Ce-Type", source.Type)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, req)
				return w
			}
			require.Equal(t, tc.code, call().Code)
			if tc.kind != "" {
				require.Len(t, p.events, 1)
				require.Equal(t, tc.kind, p.events[0].Type)
				_, err := events.MarshalEnvelope(p.events[0])
				require.NoError(t, err)
				call()
				require.Equal(t, p.events[0].EventId, p.events[1].EventId)
				require.Equal(t, p.events[0].OccurredAt, p.events[1].OccurredAt)
			}
			if tc.headerID != "" {
				require.Zero(t, f.calls)
			}
		})
	}
}

func TestHandler_RejectsMalformedCloudEventsBeforeFetching(t *testing.T) {
	fixture, err := os.ReadFile("../../../../tests/contract/fixtures/resume.uploaded.json")
	require.NoError(t, err)
	for _, tc := range []struct {
		name, method, path, body, header, value string
		code                                    int
	}{
		{name: "wrong method", method: "GET", path: "/", code: 405},
		{name: "wrong path", method: "POST", path: "/other", code: 404},
		{name: "malformed body", body: "{", code: 400},
		{name: "oversized body", body: string(bytes.Repeat([]byte("x"), 65537)), code: 400},
		{name: "wrong spec", header: "Ce-Specversion", value: "0.3", code: 400},
		{name: "wrong MIME", header: "Content-Type", value: "text/plain", code: 400},
		{name: "missing source", header: "Ce-Source", value: "", code: 400},
		{name: "wrong producer", body: string(bytes.Replace(fixture, []byte(`"producer": "resume"`), []byte(`"producer": "other"`), 1)), code: 400},
		{name: "wrong data", body: string(bytes.Replace(fixture, []byte("ResumeUploaded"), []byte("ResumeValidated"), 1)), code: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &files{}
			p := &publisher{}
			h := validator.Handler{Files: f, Events: p, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			method := tc.method
			if method == "" {
				method = "POST"
			}
			path := tc.path
			if path == "" {
				path = "/"
			}
			body := fixture
			if tc.body != "" {
				body = []byte(tc.body)
			}
			req := httptest.NewRequest(method, path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Ce-Specversion", "1.0")
			req.Header.Set("Ce-Source", "/jobtrack/rabbitmq")
			req.Header.Set("Ce-Id", "00000000-0000-4000-8000-000000000001")
			req.Header.Set("Ce-Type", "resume.uploaded")
			if tc.header != "" {
				req.Header.Set(tc.header, tc.value)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			require.Equal(t, tc.code, w.Code)
			require.Zero(t, f.calls)
			require.Empty(t, p.events)
		})
	}
}

// This is a documented acceptance limitation, not a claim that the PDF is invalid.
func TestValidate_CompressedCatalogIsExplicitlyUnsupported(t *testing.T) {
	content := testpdf.CompressedCatalog()
	require.NotContains(t, string(content), "/Type /Catalog")
	require.Contains(t, string(content), "/Type /ObjStm")
	require.Equal(t, validator.UnsupportedLayout, validator.Validate("application/pdf", content))
}
