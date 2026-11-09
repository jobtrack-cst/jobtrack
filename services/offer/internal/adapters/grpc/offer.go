package grpc

import (
	"context"
	"errors"
	commonv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/common/v1"
	offerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/offer/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/authctx"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OfferHandler struct {
	offerv1.UnimplementedOfferServiceServer
	offers *app.Offers
}

var _ offerv1.OfferServiceServer = (*OfferHandler)(nil)

func NewOfferHandler(offers *app.Offers) *OfferHandler { return &OfferHandler{offers: offers} }

func (h *OfferHandler) MakeOffer(ctx context.Context, req *offerv1.MakeOfferRequest) (*offerv1.MakeOfferResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	o, err := h.offers.MakeOffer(ctx, caller, req.GetApplicationId(), req.GetTerms())
	if err != nil {
		return nil, toStatus(err)
	}
	return &offerv1.MakeOfferResponse{Offer: offerToProto(o)}, nil
}

func (h *OfferHandler) GetOffer(ctx context.Context, req *offerv1.GetOfferRequest) (*offerv1.GetOfferResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	o, err := h.offers.GetOffer(ctx, caller, req.GetOfferId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &offerv1.GetOfferResponse{Offer: offerToProto(o)}, nil
}

func (h *OfferHandler) AcceptOffer(ctx context.Context, req *offerv1.AcceptOfferRequest) (*offerv1.AcceptOfferResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	o, err := h.offers.AcceptOffer(ctx, caller, req.GetOfferId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &offerv1.AcceptOfferResponse{Offer: offerToProto(o)}, nil
}

func (h *OfferHandler) DeclineOffer(ctx context.Context, req *offerv1.DeclineOfferRequest) (*offerv1.DeclineOfferResponse, error) {
	caller, err := callerFrom(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	o, err := h.offers.DeclineOffer(ctx, caller, req.GetOfferId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &offerv1.DeclineOfferResponse{Offer: offerToProto(o)}, nil
}

func callerFrom(ctx context.Context) (domain.Caller, error) {
	identity, ok := authctx.FromContext(ctx)
	if !ok {
		return domain.Caller{}, domain.ErrUnauthenticated
	}
	return domain.NewCaller(identity.UserID, identity.Role)
}
func offerToProto(o domain.Offer) *offerv1.Offer {
	return &offerv1.Offer{OfferId: o.ID, ApplicationId: o.ApplicationID, EmployerId: o.EmployerID, ApplicantId: o.ApplicantID, Terms: o.Terms, Status: statusToProto(o.Status), Timestamps: &commonv1.ResourceTimestamps{CreatedAt: timestamppb.New(o.CreatedAt), UpdatedAt: timestamppb.New(o.UpdatedAt)}}
}
func statusToProto(s domain.OfferStatus) offerv1.OfferStatus {
	switch s {
	case domain.OfferStatusPending:
		return offerv1.OfferStatus_OFFER_STATUS_PENDING
	case domain.OfferStatusAccepted:
		return offerv1.OfferStatus_OFFER_STATUS_ACCEPTED
	case domain.OfferStatusDeclined:
		return offerv1.OfferStatus_OFFER_STATUS_DECLINED
	default:
		return offerv1.OfferStatus_OFFER_STATUS_UNSPECIFIED
	}
}
func toStatus(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrPermissionDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrInvalidOfferID), errors.Is(err, domain.ErrInvalidApplicationID), errors.Is(err, domain.ErrInvalidTerms):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrOfferNotFound), errors.Is(err, domain.ErrParticipantsNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrOfferExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrOfferNotPending):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
