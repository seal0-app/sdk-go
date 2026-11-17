package seal0

import "fmt"

// APIError is a non-2xx response from the API, decoded from the error
// envelope. Compare against the sentinels below with errors.Is.
type APIError struct {
	Status    int    // HTTP status code
	Code      string // stable snake_case error code
	Message   string
	Field     string // offending input field, when known
	RequestID string // quote this when contacting support
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("seal0: %d %s: %s", e.Status, e.Code, e.Message)
	if e.Field != "" {
		msg += " (field " + e.Field + ")"
	}
	if e.RequestID != "" {
		msg += " [" + e.RequestID + "]"
	}
	return msg
}

// Is matches another *APIError by Code, so errors.Is(err, ErrCustomerNotFound)
// works for any response carrying that code.
func (e *APIError) Is(target error) bool {
	t, ok := target.(*APIError)
	return ok && t.Code != "" && t.Code == e.Code
}

// Sentinel errors for codes callers commonly handle. Any other code is still
// available on *APIError.
//
// Unknown customers and feature keys are errors, while a missing entitlement
// is a normal result with Granted=false. Treat any error as "not granted"
// (fail closed) unless you have a reason not to.
var (
	ErrInvalidAPIKey        = &APIError{Code: "invalid_api_key"}
	ErrInsufficientScope    = &APIError{Code: "insufficient_scope"}
	ErrCustomerNotFound     = &APIError{Code: "customer_not_found"}
	ErrFeatureNotFound      = &APIError{Code: "feature_not_found"}
	ErrPlanNotFound         = &APIError{Code: "plan_not_found"}
	ErrPlanVersionNotFound  = &APIError{Code: "plan_version_not_found"}
	ErrDraftNotFound        = &APIError{Code: "draft_not_found"}
	ErrSubscriptionNotFound = &APIError{Code: "subscription_not_found"}
	ErrOverrideNotFound     = &APIError{Code: "override_not_found"}
	ErrFeatureKeyTaken      = &APIError{Code: "feature_key_taken"}
	ErrPlanKeyTaken         = &APIError{Code: "plan_key_taken"}
	ErrOverrideOverlap      = &APIError{Code: "override_overlap"}
	ErrNoDraft              = &APIError{Code: "no_draft"}
	ErrSubscriptionReplaced = &APIError{Code: "subscription_not_current"}
	ErrInvalidSubscription  = &APIError{Code: "invalid_subscription_state"}
	ErrIdempotencyKeyReused = &APIError{Code: "idempotency_key_reused"}
	ErrFeatureNotMetered    = &APIError{Code: "feature_not_metered"}
	// ErrUsageLimitExceeded: the Account is over its Seal0 evaluation limit
	// (429). It lasts until the usage period resets, so it is never retried.
	ErrUsageLimitExceeded = &APIError{Code: "usage_limit_exceeded"}
)
