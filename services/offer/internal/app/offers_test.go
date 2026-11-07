package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/ports/mocks"
	"go.uber.org/mock/gomock"
)

const (
	applicationID = "aaaaaaaa-1111-4111-8111-111111111111"
	employerID    = "bbbbbbbb-2222-4222-8222-222222222222"
	applicantID   = "cccccccc-3333-4333-8333-333333333333"
	offerID       = "dddddddd-4444-4444-8444-444444444444"
	strangerID    = "eeeeeeee-5555-4555-8555-555555555555"
)

var infrastructure = errors.New("database unavailable")
var at = time.Date(2026, 11, 8, 12, 0, 0, 0, time.FixedZone("Bhutan", 21600))

func pending() domain.Offer {
	return domain.Offer{ID: offerID, ApplicationID: applicationID, EmployerID: employerID, ApplicantID: applicantID, Terms: "terms", Status: domain.OfferStatusPending, CreatedAt: at.UTC(), UpdatedAt: at.UTC()}
}

func TestOffers_MakeOffer(t *testing.T) {
	for _, name := range []string{"success", "applicant", "admin", "unauthenticated", "invalid application", "invalid terms", "missing participants", "participant failure", "wrong owner", "duplicate", "create failure"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockOfferRepository(ctrl)
			participants := mocks.NewMockParticipantRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			caller := domain.Caller{UserID: employerID, Role: "employer"}
			id, terms := applicationID, "  terms  "
			var wantErr error
			switch name {
			case "applicant":
				caller.Role = "applicant"
				wantErr = domain.ErrPermissionDenied
			case "admin":
				caller.Role = "admin"
				wantErr = domain.ErrPermissionDenied
			case "unauthenticated":
				caller.UserID = ""
				wantErr = domain.ErrUnauthenticated
			case "invalid application":
				id = "bad"
				wantErr = domain.ErrInvalidApplicationID
			case "invalid terms":
				terms = " \n "
				wantErr = domain.ErrInvalidTerms
			default:
				p := domain.ApplicationParticipants{ApplicationID: applicationID, EmployerID: employerID, ApplicantID: applicantID}
				var perr error
				switch name {
				case "missing participants":
					perr = domain.ErrParticipantsNotFound
				case "participant failure":
					perr = infrastructure
				case "wrong owner":
					p.EmployerID = strangerID
					wantErr = domain.ErrPermissionDenied
				}
				participants.EXPECT().GetByApplicationID(gomock.Any(), applicationID).Return(p, perr)
				if perr != nil {
					wantErr = perr
				} else if name != "wrong owner" {
					clock.EXPECT().Now().Return(at)
					switch name {
					case "duplicate":
						wantErr = domain.ErrOfferExists
					case "create failure":
						wantErr = infrastructure
					}
					repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, o domain.Offer) (domain.Offer, error) {
						if _, err := uuid.Parse(o.ID); err != nil {
							t.Fatal(err)
						}
						want := pending()
						want.ID = o.ID
						if o != want {
							t.Fatalf("got %+v want %+v", o, want)
						}
						if wantErr != nil {
							return domain.Offer{}, wantErr
						}
						return o, nil
					})
				}
			}
			got, err := app.NewOffers(repo, participants, clock).MakeOffer(context.Background(), caller, id, terms)
			if !errors.Is(err, wantErr) {
				t.Fatalf("got %v want %v", err, wantErr)
			}
			if err != nil && got != (domain.Offer{}) {
				t.Fatal("returned offer on failure")
			}
		})
	}
}

func TestOffers_GetOffer_Authorization(t *testing.T) {
	for _, tc := range []struct {
		name, user, role string
		want             error
	}{
		{"applicant", applicantID, "applicant", nil}, {"employer", employerID, "employer", nil},
		{"uppercase caller", strings.ToUpper(applicantID), "applicant", nil},
		{"unrelated applicant", strangerID, "applicant", domain.ErrPermissionDenied},
		{"unrelated employer", strangerID, "employer", domain.ErrPermissionDenied},
		{"admin", applicantID, "admin", domain.ErrPermissionDenied},
		{"wrong role matching ID", applicantID, "employer", domain.ErrPermissionDenied},
		{"missing identity", "", "applicant", domain.ErrUnauthenticated},
		{"invalid role", applicantID, "other", domain.ErrUnauthenticated},
		{"unknown offer", applicantID, "applicant", domain.ErrOfferNotFound},
		{"database failure", applicantID, "applicant", infrastructure},
		{"invalid ID", applicantID, "applicant", domain.ErrInvalidOfferID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockOfferRepository(ctrl)
			id := offerID
			if tc.name == "invalid ID" {
				id = "bad"
			}
			if tc.want != domain.ErrUnauthenticated && tc.want != domain.ErrInvalidOfferID {
				var err error
				if tc.name == "unknown offer" || tc.name == "database failure" {
					err = tc.want
				}
				repo.EXPECT().GetByID(gomock.Any(), offerID).Return(pending(), err)
			}
			got, err := app.NewOffers(repo, nil, nil).GetOffer(context.Background(), domain.Caller{UserID: tc.user, Role: tc.role}, id)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if err == nil && got != pending() {
				t.Fatal(got)
			}
		})
	}
}

func TestOffers_Decisions(t *testing.T) {
	for _, accept := range []bool{true, false} {
		action := "decline"
		if accept {
			action = "accept"
		}
		for _, name := range []string{"success", "employer", "admin", "missing identity", "wrong applicant", "unknown", "invalid ID", "accepted", "declined", "lookup failure", "write failure", "concurrent loser"} {
			t.Run(action+"/"+name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				repo := mocks.NewMockOfferRepository(ctrl)
				clock := mocks.NewMockClock(ctrl)
				caller := domain.Caller{UserID: applicantID, Role: "applicant"}
				id := offerID
				var wantErr error
				switch name {
				case "employer", "admin":
					caller.Role = name
					wantErr = domain.ErrPermissionDenied
				case "missing identity":
					caller.UserID = ""
					wantErr = domain.ErrUnauthenticated
				case "invalid ID":
					id = "bad"
					wantErr = domain.ErrInvalidOfferID
				default:
					o := pending()
					var readErr error
					switch name {
					case "wrong applicant":
						caller.UserID = strangerID
						wantErr = domain.ErrPermissionDenied
					case "unknown":
						readErr = domain.ErrOfferNotFound
						wantErr = readErr
					case "lookup failure":
						readErr = infrastructure
						wantErr = readErr
					case "accepted":
						o.Status = domain.OfferStatusAccepted
						wantErr = domain.ErrOfferNotPending
					case "declined":
						o.Status = domain.OfferStatusDeclined
						wantErr = domain.ErrOfferNotPending
					}
					repo.EXPECT().GetByID(gomock.Any(), offerID).Return(o, readErr)
					if wantErr == nil {
						next := domain.OfferStatusDeclined
						if accept {
							next = domain.OfferStatusAccepted
						}
						clock.EXPECT().Now().Return(at)
						if name == "write failure" {
							wantErr = infrastructure
						}
						if name == "concurrent loser" {
							wantErr = domain.ErrOfferNotPending
						}
						o.Status = next
						repo.EXPECT().Decide(gomock.Any(), offerID, applicantID, next, at.UTC()).Return(o, wantErr)
					}
				}
				svc := app.NewOffers(repo, nil, clock)
				var got domain.Offer
				var err error
				if accept {
					got, err = svc.AcceptOffer(context.Background(), caller, id)
				} else {
					got, err = svc.DeclineOffer(context.Background(), caller, id)
				}
				if !errors.Is(err, wantErr) {
					t.Fatalf("got %v want %v", err, wantErr)
				}
				if err == nil {
					want := domain.OfferStatusDeclined
					if accept {
						want = domain.OfferStatusAccepted
					}
					if got.Status != want {
						t.Fatal(got)
					}
				}
			})
		}
	}
}

func TestOffers_MakeOffer_InvalidTermsNeverReachRepositories(t *testing.T) {
	for _, tc := range []struct{ name, terms string }{
		{"embedded NUL", "terms\x00text"},
		{"trailing NUL", "terms\x00"},
		{"empty", ""},
		{"whitespace", " \n\t "},
		{"over Unicode limit", strings.Repeat("ཀ", 2001)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := mocks.NewMockOfferRepository(ctrl)
			participants := mocks.NewMockParticipantRepository(ctrl)
			clock := mocks.NewMockClock(ctrl)
			// No expectations: any participant lookup, persistence or clock call fails.
			svc := app.NewOffers(repo, participants, clock)
			got, err := svc.MakeOffer(context.Background(), domain.Caller{UserID: employerID, Role: "employer"}, applicationID, tc.terms)
			if !errors.Is(err, domain.ErrInvalidTerms) || got != (domain.Offer{}) {
				t.Fatalf("got %+v, %v; want empty offer and ErrInvalidTerms", got, err)
			}
		})
	}
}
