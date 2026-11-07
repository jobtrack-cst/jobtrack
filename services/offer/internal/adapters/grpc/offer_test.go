package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	offerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/offer/v1"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const offerID = "aaaaaaaa-1111-4111-8111-111111111111"
const applicantID = "bbbbbbbb-2222-4222-8222-222222222222"
const employerID = "cccccccc-3333-4333-8333-333333333333"

func identity(id, role string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", id, "x-user-role", role))
}

func TestOfferHandler_ErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{
		{domain.ErrUnauthenticated, codes.Unauthenticated}, {domain.ErrPermissionDenied, codes.PermissionDenied},
		{domain.ErrInvalidApplicationID, codes.InvalidArgument}, {domain.ErrInvalidOfferID, codes.InvalidArgument}, {domain.ErrInvalidTerms, codes.InvalidArgument},
		{domain.ErrParticipantsNotFound, codes.NotFound}, {domain.ErrOfferNotFound, codes.NotFound}, {domain.ErrOfferExists, codes.AlreadyExists},
		{domain.ErrOfferNotPending, codes.FailedPrecondition}, {context.Canceled, codes.Canceled}, {context.DeadlineExceeded, codes.DeadlineExceeded},
		{errors.New("private connection password"), codes.Internal},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			got := toStatus(tc.err)
			if status.Code(got) != tc.code {
				t.Fatal(got)
			}
			if tc.code == codes.Internal && status.Convert(got).Message() != "internal error" {
				t.Fatal(got)
			}
		})
	}
}

func TestOfferHandler_IdentityRequiredForEveryRPC(t *testing.T) {
	h := NewOfferHandler(app.NewOffers(nil, nil, nil))
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{"missing", context.Background()}, {"invalid UUID", identity("bad", "applicant")},
		{"invalid role", identity(applicantID, "other")}, {"missing role", identity(applicantID, "")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, a := h.MakeOffer(tc.ctx, &offerv1.MakeOfferRequest{})
			_, b := h.GetOffer(tc.ctx, &offerv1.GetOfferRequest{})
			_, c := h.AcceptOffer(tc.ctx, &offerv1.AcceptOfferRequest{})
			_, d := h.DeclineOffer(tc.ctx, &offerv1.DeclineOfferRequest{})
			for _, err := range []error{a, b, c, d} {
				if status.Code(err) != codes.Unauthenticated {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestOfferHandler_NilAndInvalidRequests(t *testing.T) {
	h := NewOfferHandler(app.NewOffers(nil, nil, nil))
	ctx := identity(applicantID, "applicant")
	_, a := h.MakeOffer(ctx, nil)
	_, b := h.GetOffer(ctx, nil)
	_, c := h.AcceptOffer(ctx, nil)
	_, d := h.DeclineOffer(ctx, nil)
	for _, err := range []error{a, b, c, d} {
		if status.Code(err) != codes.InvalidArgument {
			t.Fatal(err)
		}
	}
	_, a = h.MakeOffer(identity(employerID, "employer"), &offerv1.MakeOfferRequest{ApplicationId: "bad", Terms: "terms"})
	_, b = h.GetOffer(ctx, &offerv1.GetOfferRequest{OfferId: "bad"})
	_, c = h.AcceptOffer(ctx, &offerv1.AcceptOfferRequest{OfferId: "bad"})
	_, d = h.DeclineOffer(ctx, &offerv1.DeclineOfferRequest{OfferId: "bad"})
	for _, err := range []error{a, b, c, d} {
		if status.Code(err) != codes.InvalidArgument {
			t.Fatal(err)
		}
	}
}
func TestOfferHandler_ResponsesAndAuthorization(t *testing.T) {
	for _, method := range []string{"make", "get applicant", "get employer", "get admin", "get unrelated", "accept", "decline", "accept employer", "decline employer", "make applicant"} {
		t.Run(method, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockOfferRepository(ctrl)
			p := mocks.NewMockParticipantRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			at := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
			o := domain.Offer{ID: offerID, ApplicationID: offerID, ApplicantID: applicantID, EmployerID: employerID, Terms: "terms", Status: domain.OfferStatusPending, CreatedAt: at, UpdatedAt: at}
			h := NewOfferHandler(app.NewOffers(repo, p, clock))
			ctx := identity(applicantID, "applicant")
			wantStatus := offerv1.OfferStatus_OFFER_STATUS_PENDING
			if method == "accept" {
				wantStatus = offerv1.OfferStatus_OFFER_STATUS_ACCEPTED
			} else if method == "decline" {
				wantStatus = offerv1.OfferStatus_OFFER_STATUS_DECLINED
			}
			var got *offerv1.Offer
			var err error
			switch method {
			case "make":
				p.EXPECT().GetByApplicationID(gomock.Any(), offerID).Return(domain.ApplicationParticipants{ApplicationID: offerID, ApplicantID: applicantID, EmployerID: employerID}, nil)
				clock.EXPECT().Now().Return(at)
				repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(o, nil)
				r, e := h.MakeOffer(identity(employerID, "employer"), &offerv1.MakeOfferRequest{ApplicationId: offerID, Terms: " terms "})
				err = e
				got = r.GetOffer()
			case "make applicant":
				_, err = h.MakeOffer(ctx, &offerv1.MakeOfferRequest{ApplicationId: offerID, Terms: "terms"})
			case "accept employer":
				_, err = h.AcceptOffer(identity(employerID, "employer"), &offerv1.AcceptOfferRequest{OfferId: offerID})
			case "decline employer":
				_, err = h.DeclineOffer(identity(employerID, "employer"), &offerv1.DeclineOfferRequest{OfferId: offerID})
			default:
				repo.EXPECT().GetByID(gomock.Any(), offerID).Return(o, nil)
				switch method {
				case "accept", "decline":
					clock.EXPECT().Now().Return(at)
					next := domain.OfferStatusAccepted
					if method == "decline" {
						next = domain.OfferStatusDeclined
					}
					o.Status = next
					repo.EXPECT().Decide(gomock.Any(), offerID, applicantID, next, at).Return(o, nil)
					if method == "accept" {
						r, e := h.AcceptOffer(ctx, &offerv1.AcceptOfferRequest{OfferId: offerID})
						err = e
						got = r.GetOffer()
					} else {
						r, e := h.DeclineOffer(ctx, &offerv1.DeclineOfferRequest{OfferId: offerID})
						err = e
						got = r.GetOffer()
					}
				default:
					if method == "get employer" {
						ctx = identity(employerID, "employer")
					}
					if method == "get admin" {
						ctx = identity(applicantID, "admin")
					}
					if method == "get unrelated" {
						ctx = identity(offerID, "applicant")
					}
					r, e := h.GetOffer(ctx, &offerv1.GetOfferRequest{OfferId: offerID})
					err = e
					got = r.GetOffer()
				}
			}
			denied := method == "get admin" || method == "get unrelated" || method == "accept employer" || method == "decline employer" || method == "make applicant"
			if denied {
				if status.Code(err) != codes.PermissionDenied {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.GetOfferId() != o.ID || got.GetApplicationId() != o.ApplicationID || got.GetApplicantId() != applicantID || got.GetEmployerId() != employerID || got.GetTerms() != "terms" || got.GetStatus() != wantStatus || !got.GetTimestamps().GetCreatedAt().AsTime().Equal(at) || !got.GetTimestamps().GetUpdatedAt().AsTime().Equal(at) {
				t.Fatalf("incorrect response: %v", got)
			}
		})
	}
}

func TestOfferHandler_MakeOffer_InvalidTermsNeverReachRepositories(t *testing.T) {
	for _, tc := range []struct{ name, terms string }{
		{"embedded NUL", "terms\x00text"},
		{"trailing NUL", "terms\x00"},
		{"empty", ""},
		{"whitespace", " \n\t "},
		{"over Unicode limit", strings.Repeat("ཀ", 2001)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			// No repository expectations: malformed terms must fail before any DB call.
			repo := mocks.NewMockOfferRepository(ctrl)
			participants := mocks.NewMockParticipantRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			handler := NewOfferHandler(app.NewOffers(repo, participants, clock))
			response, err := handler.MakeOffer(identity(employerID, "employer"), &offerv1.MakeOfferRequest{ApplicationId: offerID, Terms: tc.terms})
			if response != nil || status.Code(err) != codes.InvalidArgument {
				t.Fatalf("got %v, %v; want nil response and INVALID_ARGUMENT", response, err)
			}
		})
	}
}
