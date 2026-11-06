package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	authv1 "github.com/jobtrack-cst/jobtrack/gen/go/jobtrack/auth/v1"
	"github.com/jobtrack-cst/jobtrack/pkg/logging"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/app"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/domain"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/ports"
	"github.com/jobtrack-cst/jobtrack/services/auth/internal/ports/mocks"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	testJWTSecret     = "synthetic-test-secret-32-bytes!!"
	testAdminPassword = "synthetic-admin-pass"
	testAdminEmail    = "coordinator@cst.edu.bt"
)

// stubUsers satisfies the port in startup-order tests that never call it.
type stubUsers struct{ ports.UserRepository }

func authConfig() config {
	return config{serviceName: "auth", jwtSecret: testJWTSecret, adminEmail: testAdminEmail, adminPassword: testAdminPassword}
}

func TestSetupAuth_SeedsAdminWithoutLoggingSecrets(t *testing.T) {
	users := mocks.NewMockUserRepository(gomock.NewController(t))
	users.EXPECT().ExistsWithRole(gomock.Any(), domain.RoleAdmin).Return(false, nil)
	users.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u domain.User) (domain.User, error) {
		if u.Email != testAdminEmail || u.Role != domain.RoleAdmin || u.FullName != app.AdminFullName {
			t.Errorf("unexpected admin %+v", u)
		}
		if u.PasswordHash == testAdminPassword || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(testAdminPassword)) != nil {
			t.Error("admin password was not stored as a bcrypt hash")
		}
		return u, nil
	})
	var logs bytes.Buffer
	register, err := setupAuth(context.Background(), authConfig(), logging.New("auth", &logs), users, bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	register(server)
	info := server.GetServiceInfo()
	for _, name := range []string{"jobtrack.auth.v1.AuthService", "jobtrack.auth.v1.AuthInternalService"} {
		if _, ok := info[name]; !ok {
			t.Errorf("%s not registered", name)
		}
	}
	if strings.Contains(logs.String(), testAdminEmail) || strings.Contains(logs.String(), testAdminPassword) {
		t.Fatalf("admin values logged: %s", logs.String())
	}
}

func TestSetupAuth_Failures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		cfg     func(c *config)
		setup   func(u *mocks.MockUserRepository)
		wantMsg string
	}{
		{
			name: "existing admin is never updated",
			setup: func(u *mocks.MockUserRepository) {
				u.EXPECT().ExistsWithRole(gomock.Any(), domain.RoleAdmin).Return(true, nil)
			},
		},
		{name: "weak JWT secret", cfg: func(c *config) { c.jwtSecret = "short" }, wantMsg: "JWT_SECRET"},
		{
			name: "admin email used by another account",
			setup: func(u *mocks.MockUserRepository) {
				u.EXPECT().ExistsWithRole(gomock.Any(), domain.RoleAdmin).Return(false, nil)
				u.EXPECT().Create(gomock.Any(), gomock.Any()).Return(domain.User{}, domain.ErrEmailTaken)
			},
			wantMsg: "ADMIN_EMAIL",
		},
		{
			name: "database failure",
			setup: func(u *mocks.MockUserRepository) {
				u.EXPECT().ExistsWithRole(gomock.Any(), domain.RoleAdmin).Return(false, errors.New("private SQL"))
			},
			wantMsg: "seed admin failed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			users := mocks.NewMockUserRepository(gomock.NewController(t))
			if tt.setup != nil {
				tt.setup(users)
			}
			cfg := authConfig()
			if tt.cfg != nil {
				tt.cfg(&cfg)
			}
			_, err := setupAuth(context.Background(), cfg, logging.New("auth", &bytes.Buffer{}), users, bcrypt.MinCost)
			if tt.wantMsg == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) || strings.Contains(err.Error(), "private") {
				t.Fatalf("err=%v, want message containing %q", err, tt.wantMsg)
			}
		})
	}
}

// Proves the registered services answer over a real gRPC connection.
func TestServe_AnswersAuthRPCs(t *testing.T) {
	users := mocks.NewMockUserRepository(gomock.NewController(t))
	users.EXPECT().ExistsWithRole(gomock.Any(), domain.RoleAdmin).Return(true, nil)
	users.EXPECT().GetByEmail(gomock.Any(), "nobody@cst.edu.bt").Return(domain.User{}, domain.ErrUserNotFound)
	register, err := setupAuth(context.Background(), authConfig(), logging.New("auth", &bytes.Buffer{}), users, bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	grpcListener, httpListener := listenForTest(t), listenForTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, logging.New("auth", &bytes.Buffer{}), app.NewHealth(), register, grpcListener, httpListener)
	}()
	conn, err := grpc.NewClient(grpcListener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rpcCancel()

	_, err = authv1.NewAuthServiceClient(conn).Login(rpcCtx, &authv1.LoginRequest{Email: "nobody@cst.edu.bt", Password: "password123"})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("Login code=%v, want Unauthenticated", status.Code(err))
	}
	_, err = authv1.NewAuthInternalServiceClient(conn).GetUser(rpcCtx, &authv1.GetUserRequest{UserId: "abc"})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("GetUser code=%v, want InvalidArgument", status.Code(err))
	}
	cancel()
	if err := awaitServer(t, done); err != nil {
		t.Fatal(err)
	}
}
