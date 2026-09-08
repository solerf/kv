package k8s

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// These sentinels mirror the client-go/apiserver error taxonomy so callers can
// branch with errors.Is without importing client-go. classify wraps with %w, so
// errors.Is matches the sentinel and the underlying error alike.
var (
	// 4xx — the request itself is the problem; retrying unchanged won't help.
	ErrNotFound           = errors.New("not found")            // 404
	ErrForbidden          = errors.New("forbidden")            // 403
	ErrUnauthorized       = errors.New("unauthorized")         // 401
	ErrConflict           = errors.New("conflict")             // 409
	ErrAlreadyExists      = errors.New("already exists")       // 409
	ErrInvalid            = errors.New("invalid")              // 422
	ErrBadRequest         = errors.New("bad request")          // 400
	ErrGone               = errors.New("gone")                 // 410
	ErrResourceExpired    = errors.New("resource expired")     // 410
	ErrMethodNotSupported = errors.New("method not supported") // 405
	ErrTooManyRequests    = errors.New("too many requests")    // 429

	// 5xx / timeouts — transient; the same request may succeed on retry.
	ErrServerTimeout      = errors.New("server timeout")        // 504
	ErrServiceUnavailable = errors.New("service unavailable")   // 503
	ErrInternal           = errors.New("internal server error") // 500

	// Not apiserver Status errors: the request never got a reply.
	ErrCanceled    = errors.New("request canceled")    // context canceled / deadline exceeded
	ErrUnreachable = errors.New("cluster unreachable") // transport failure (DNS, TCP, TLS)
)

// classify maps a client-go error to a package sentinel, wrapping the original
// (%w). A nil error stays nil; an unmatched error is returned unchanged.
func classify(err error) error {
	if err == nil {
		return nil
	}

	// Checked first: cancellation can surface as a *url.Error wrapping it.
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%w: %w", ErrCanceled, err)
	case apierrors.IsNotFound(err):
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case apierrors.IsForbidden(err):
		return fmt.Errorf("%w: %w", ErrForbidden, err)
	case apierrors.IsUnauthorized(err):
		return fmt.Errorf("%w: %w", ErrUnauthorized, err)
	case apierrors.IsConflict(err):
		return fmt.Errorf("%w: %w", ErrConflict, err)
	case apierrors.IsAlreadyExists(err):
		return fmt.Errorf("%w: %w", ErrAlreadyExists, err)
	case apierrors.IsInvalid(err):
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	case apierrors.IsBadRequest(err):
		return fmt.Errorf("%w: %w", ErrBadRequest, err)
	case apierrors.IsGone(err):
		return fmt.Errorf("%w: %w", ErrGone, err)
	case apierrors.IsResourceExpired(err):
		return fmt.Errorf("%w: %w", ErrResourceExpired, err)
	case apierrors.IsMethodNotSupported(err):
		return fmt.Errorf("%w: %w", ErrMethodNotSupported, err)
	case apierrors.IsTooManyRequests(err):
		return fmt.Errorf("%w: %w", ErrTooManyRequests, err)
	case apierrors.IsServerTimeout(err):
		return fmt.Errorf("%w: %w", ErrServerTimeout, err)
	case apierrors.IsServiceUnavailable(err):
		return fmt.Errorf("%w: %w", ErrServiceUnavailable, err)
	case apierrors.IsInternalError(err):
		return fmt.Errorf("%w: %w", ErrInternal, err)
	}

	// Transport failure: never reached the apiserver, so not a Status error.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}

	return err
}
