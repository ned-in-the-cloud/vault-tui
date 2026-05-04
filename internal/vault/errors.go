package vault

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
)

// Sentinel errors used by the TUI to drive UX flows. Production calls in
// this package wrap raw Vault API errors with one of these so screens do
// not need to inspect HTTP status codes.
var (
	ErrPermissionDenied = errors.New("vault: permission denied")
	ErrNotFound         = errors.New("vault: not found")
	ErrServerSealed     = errors.New("vault: server sealed")
	ErrConnectionFailed = errors.New("vault: connection failed")
	ErrInvalidToken     = errors.New("vault: invalid or expired token")
	ErrCASRequired      = errors.New("vault: check-and-set required")
)

// classifyError converts an error from the Vault API into one of the
// sentinel errors above when possible. The original error is wrapped so
// callers can use errors.Unwrap if they need the underlying detail.
func classifyError(err error) error {
	if err == nil {
		return nil
	}
	var respErr *vaultapi.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.StatusCode {
		case http.StatusForbidden:
			return fmt.Errorf("%w: %s", ErrPermissionDenied, summarize(respErr))
		case http.StatusNotFound:
			return fmt.Errorf("%w: %s", ErrNotFound, summarize(respErr))
		case http.StatusServiceUnavailable:
			return fmt.Errorf("%w: %s", ErrServerSealed, summarize(respErr))
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %s", ErrInvalidToken, summarize(respErr))
		case http.StatusBadRequest:
			if mentionsCAS(respErr) {
				return fmt.Errorf("%w: %s", ErrCASRequired, summarize(respErr))
			}
		}
		return err
	}
	// Connection-level failures: the api package returns wrapped *url.Error
	// values. We treat any non-ResponseError that mentions a network verb
	// as a connection failure.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "tls") ||
		strings.Contains(msg, "x509") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "i/o timeout") {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	return err
}

func summarize(r *vaultapi.ResponseError) string {
	if r == nil {
		return ""
	}
	if len(r.Errors) > 0 {
		return strings.Join(r.Errors, "; ")
	}
	return r.Error()
}

func mentionsCAS(r *vaultapi.ResponseError) bool {
	for _, msg := range r.Errors {
		if strings.Contains(strings.ToLower(msg), "check-and-set") {
			return true
		}
	}
	return false
}
