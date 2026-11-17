package seal0

// Known values of string fields (x-known-values in the API contract). The API
// may add values in the future, so code must handle unknown ones; these
// constants never cause decoding to fail.

// Subscription statuses.
const (
	StatusTrialing SubscriptionStatus = "trialing"
	StatusActive   SubscriptionStatus = "active"
	StatusPastDue  SubscriptionStatus = "past_due"
	StatusPaused   SubscriptionStatus = "paused"
	StatusCanceled SubscriptionStatus = "canceled"
	StatusExpired  SubscriptionStatus = "expired"
)

// Feature types.
const (
	FeatureTypeBoolean FeatureType = "boolean"
	FeatureTypeNumber  FeatureType = "number"
	FeatureTypeEnum    FeatureType = "enum"
	FeatureTypeMetered FeatureType = "metered" // a limit per usage period (ADR-0029)
)

// Entitlement sources (Entitlement.Source).
const (
	SourceOverride = "override"
	SourcePlan     = "plan"
	SourceDefault  = "default"
	SourceNone     = "none"
)

// Plan version statuses (PlanVersion.Status, PlanVersionSummary.Status).
const (
	PlanVersionDraft     = "draft"
	PlanVersionPublished = "published"
)
