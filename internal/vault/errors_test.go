package vault

import (
	"context"
	"errors"
	"net/http"
	"testing"

	vaultapi "github.com/hashicorp/vault/api"
)

func TestClassifyResponseError(t *testing.T) {
	cases := []struct {
		name string
		code int
		want error
	}{
		{"forbidden", http.StatusForbidden, ErrPermissionDenied},
		{"notfound", http.StatusNotFound, ErrNotFound},
		{"sealed", http.StatusServiceUnavailable, ErrServerSealed},
		{"unauth", http.StatusUnauthorized, ErrInvalidToken},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := &vaultapi.ResponseError{StatusCode: tc.code, Errors: []string{"x"}}
			got := classifyError(err)
			if !errors.Is(got, tc.want) {
				t.Fatalf("classifyError(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

func TestClassifyConnectionError(t *testing.T) {
	err := errors.New("dial tcp: connection refused")
	got := classifyError(err)
	if !errors.Is(got, ErrConnectionFailed) {
		t.Fatalf("got %v, want connection failed wrapper", got)
	}
}

func TestMockClientImplementsClient(t *testing.T) {
	var _ Client = (*MockClient)(nil)
}

func TestMockClientCallsHook(t *testing.T) {
	called := false
	m := &MockClient{HealthFn: func(ctx context.Context) (*HealthInfo, error) {
		called = true
		return &HealthInfo{Initialized: true}, nil
	}}
	h, err := m.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !called || !h.Initialized {
		t.Fatalf("hook not invoked correctly")
	}
}
