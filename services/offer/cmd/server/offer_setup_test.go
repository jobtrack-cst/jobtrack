package main

import (
	"context"
	"io"
	"testing"
	"time"

	offerv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/offer/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/offer/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRegisterOffer_PublicServiceAndUTCClock(t *testing.T) {
	server := grpc.NewServer()
	registerOffer(nil, nil)(server)
	info := server.GetServiceInfo()
	if _, ok := info["jobtrack.offer.v1.OfferService"]; !ok || len(info) != 1 {
		t.Fatal(info)
	}
	if (systemClock{}).Now().Location() != time.UTC {
		t.Fatal("clock is not UTC")
	}
}
func TestServe_OfferRPCUsesForwardedIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockOfferRepository(ctrl)
	id := "aaaaaaaa-1111-4111-8111-111111111111"
	user := "bbbbbbbb-2222-4222-8222-222222222222"
	repo.EXPECT().GetByID(gomock.Any(), id).Return(domain.Offer{ID: id, ApplicantID: user, Status: domain.OfferStatusPending}, nil)
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("offer", io.Discard), app.NewHealth(), registerOffer(repo, nil), grpcListener, httpListener)
	}()
	t.Cleanup(func() {
		cancel()
		if err := awaitServer(t, done); err != nil {
			t.Error(err)
		}
	})
	conn, err := grpc.NewClient(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := offerv1.NewOfferServiceClient(conn)
	rpcCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
	defer stop()
	if _, err := client.GetOffer(rpcCtx, &offerv1.GetOfferRequest{OfferId: id}); status.Code(err) != codes.Unauthenticated {
		t.Fatal(err)
	}
	forwarded := metadata.AppendToOutgoingContext(rpcCtx, "x-user-id", user, "x-user-role", "applicant")
	resp, err := client.GetOffer(forwarded, &offerv1.GetOfferRequest{OfferId: id})
	if err != nil || resp.GetOffer().GetApplicantId() != user {
		t.Fatal(resp, err)
	}
}
