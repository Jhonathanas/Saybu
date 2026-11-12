package grpc

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	authv1 "saybu/gen/saybu/auth/v1"
	"saybu/services/auth/internal/domain"
	"saybu/services/auth/internal/service"
)

type stubService struct {
	registerRes service.AuthResult
	registerErr error

	loginRes service.AuthResult
	loginErr error

	refreshRes service.AuthResult
	refreshErr error

	logoutErr error

	user    domain.User
	userErr error

	users     []domain.User
	listTotal int64
	listPage  int
	listErr   error
}

func (s *stubService) Register(context.Context, service.RegisterInput) (service.AuthResult, error) {
	return s.registerRes, s.registerErr
}
func (s *stubService) Login(context.Context, string, string, string, string) (service.AuthResult, error) {
	return s.loginRes, s.loginErr
}
func (s *stubService) Refresh(context.Context, string, string, string) (service.AuthResult, error) {
	return s.refreshRes, s.refreshErr
}
func (s *stubService) Logout(context.Context, string) error { return s.logoutErr }
func (s *stubService) GetUser(context.Context, string) (domain.User, error) {
	return s.user, s.userErr
}
func (s *stubService) ListUsers(context.Context, int, int, string) ([]domain.User, int64, int, error) {
	return s.users, s.listTotal, s.listPage, s.listErr
}

func startServer(t *testing.T, svc Service) authv1.AuthServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	Register(srv, svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return authv1.NewAuthServiceClient(conn)
}

func TestRegisterSuccess(t *testing.T) {
	email := "siti@example.com"
	stub := &stubService{registerRes: service.AuthResult{
		User: domain.User{
			ID: "user-1", Name: "Siti", Phone: "081234567890", Email: &email,
			Role: domain.RoleCustomer, CreatedAt: time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC),
		},
		AccessToken:      "access",
		AccessExpiresIn:  15 * time.Minute,
		RefreshToken:     "refresh",
		RefreshExpiresIn: 30 * 24 * time.Hour,
	}}
	client := startServer(t, stub)

	resp, err := client.Register(context.Background(), &authv1.RegisterRequest{
		Name: "Siti", Phone: "081234567890", Email: email, Password: "rahasia123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetUser().GetId() != "user-1" || resp.GetAccessToken() != "access" {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.GetExpiresIn() != 900 {
		t.Fatalf("expires_in = %d", resp.GetExpiresIn())
	}
}

func TestRegisterValidationDetails(t *testing.T) {
	stub := &stubService{registerErr: &domain.ValidationError{Errors: []domain.FieldError{
		{Field: "phone", Message: "Nomor HP tidak valid."},
	}}}
	client := startServer(t, stub)

	_, err := client.Register(context.Background(), &authv1.RegisterRequest{})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("err = %v, code = %v", err, st.Code())
	}
	found := false
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			for _, v := range br.GetFieldViolations() {
				if v.GetField() == "phone" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("detail field phone tidak ada: %+v", st.Details())
	}
}

func TestRefreshReuseUnauthenticated(t *testing.T) {
	stub := &stubService{refreshErr: domain.ErrTokenReused}
	client := startServer(t, stub)

	_, err := client.Refresh(context.Background(), &authv1.RefreshRequest{RefreshToken: "x"})
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", st.Code())
	}
}

func TestGetUserNotFound(t *testing.T) {
	stub := &stubService{userErr: domain.ErrUserNotFound}
	client := startServer(t, stub)

	_, err := client.GetUser(context.Background(), &authv1.GetUserRequest{Id: "x"})
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("code = %v, want NotFound", st.Code())
	}
}

func TestListUsers(t *testing.T) {
	stub := &stubService{
		users:     []domain.User{{ID: "u1", Name: "Siti", Role: domain.RoleCustomer}},
		listTotal: 1,
		listPage:  1,
	}
	client := startServer(t, stub)

	resp, err := client.ListUsers(context.Background(), &authv1.ListUsersRequest{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetUsers()) != 1 || resp.GetTotal() != 1 || resp.GetLimit() != 20 {
		t.Fatalf("resp = %+v", resp)
	}
}
