package seal0

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// One method per API operation (api/bundled/openapi.sdk.yaml). Methods hold no
// entitlement logic: they encode the request, call the API and decode the
// generated response type.

func esc(s string) string { return url.PathEscape(s) }

// ---------------------------------------------------------------- evaluation

// Evaluate returns one feature's entitlement for a customer now. A missing
// entitlement is a normal result with Granted=false; an unknown customer or
// feature key returns ErrCustomerNotFound or ErrFeatureNotFound.
func (c *Client) Evaluate(ctx context.Context, customerID, featureKey string, opts ...RequestOption) (*Evaluation, error) {
	var out Evaluation
	_, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/entitlements/evaluate",
		body: EvaluateRequest{CustomerID: customerID, FeatureKey: featureKey}}, &out, opts)
	return result(&out, err)
}

// EvaluateAll returns every feature's entitlement for a customer now.
func (c *Client) EvaluateAll(ctx context.Context, customerID string, opts ...RequestOption) (*EvaluationAll, error) {
	var out EvaluationAll
	_, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/entitlements/evaluate-all",
		body: EvaluateAllRequest{CustomerID: customerID}}, &out, opts)
	return result(&out, err)
}

// --------------------------------------------------------------------- usage

// RecordUsage adds quantity (at least 1) to a customer's usage of a metered
// feature in the current period and returns the new total. Each call is one
// exact write; retries reuse the request's idempotency key, so a retried call
// is not counted twice.
func (c *Client) RecordUsage(ctx context.Context, customerID, featureKey string, quantity int64, opts ...RequestOption) (*RecordedUsage, error) {
	var out RecordedUsage
	_, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/usage",
		body: RecordUsageRequest{CustomerID: customerID, FeatureKey: featureKey, Quantity: &quantity}, mutation: true}, &out, opts)
	return result(&out, err)
}

// ------------------------------------------------------------------ features

// CreateFeature defines a feature. Its key and type cannot change later.
func (c *Client) CreateFeature(ctx context.Context, in CreateFeatureRequest, opts ...RequestOption) (*Feature, error) {
	var out Feature
	_, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/features", body: in, mutation: true}, &out, opts)
	return result(&out, err)
}

// GetFeature returns a feature by key.
func (c *Client) GetFeature(ctx context.Context, key string, opts ...RequestOption) (*Feature, error) {
	var out Feature
	_, err := c.do(ctx, request{method: http.MethodGet, path: "/v1/features/" + esc(key)}, &out, opts)
	return result(&out, err)
}

// FeaturePatch changes a feature. Unset fields are unchanged; use
// Null to clear Description or DefaultValue.
type FeaturePatch struct {
	Name           *string
	Description    Patch[string]
	DefaultValue   Patch[Value]
	AddEnumOptions []string
}

// MarshalJSON writes only the fields that are set.
func (p FeaturePatch) MarshalJSON() ([]byte, error) {
	m := patchBody{}
	if p.Name != nil {
		m["name"] = *p.Name
	}
	m.add("description", p.Description)
	m.add("default_value", p.DefaultValue)
	if p.AddEnumOptions != nil {
		m["add_enum_options"] = p.AddEnumOptions
	}
	return m.marshal()
}

// UpdateFeature changes a feature's name, description, default or adds enum
// options.
func (c *Client) UpdateFeature(ctx context.Context, key string, in FeaturePatch, opts ...RequestOption) (*Feature, error) {
	var out Feature
	_, err := c.do(ctx, request{method: http.MethodPatch, path: "/v1/features/" + esc(key), body: in, mutation: true}, &out, opts)
	return result(&out, err)
}

// ListOptions selects one page of a list.
type ListOptions struct {
	Limit  int    // 1-200; 0 means the API default (50)
	Cursor string // NextCursor from the previous page
}

func (o *ListOptions) query() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Cursor != "" {
		q.Set("cursor", o.Cursor)
	}
	return q
}

// ListFeatures returns one page of features, ordered by key. See Features
// for an iterator over all pages.
func (c *Client) ListFeatures(ctx context.Context, page *ListOptions, opts ...RequestOption) (*FeatureList, error) {
	var out FeatureList
	_, err := c.do(ctx, request{method: http.MethodGet, path: "/v1/features", query: page.query()}, &out, opts)
	return result(&out, err)
}

// --------------------------------------------------------------------- plans

// CreatePlan creates a plan with no draft.
func (c *Client) CreatePlan(ctx context.Context, in CreatePlanRequest, opts ...RequestOption) (*Plan, error) {
	var out Plan
	_, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/plans", body: in, mutation: true}, &out, opts)
	return result(&out, err)
}

// GetPlan returns a plan by key.
func (c *Client) GetPlan(ctx context.Context, key string, opts ...RequestOption) (*Plan, error) {
	var out Plan
	_, err := c.do(ctx, request{method: http.MethodGet, path: "/v1/plans/" + esc(key)}, &out, opts)
	return result(&out, err)
}

// ListPlans returns one page of plans, ordered by key.
func (c *Client) ListPlans(ctx context.Context, page *ListOptions, opts ...RequestOption) (*PlanList, error) {
	var out PlanList
	_, err := c.do(ctx, request{method: http.MethodGet, path: "/v1/plans", query: page.query()}, &out, opts)
	return result(&out, err)
}

// GetDraft returns the plan's draft with its values.
func (c *Client) GetDraft(ctx context.Context, planKey string, opts ...RequestOption) (*PlanVersion, error) {
	var out PlanVersion
	_, err := c.do(ctx, request{method: http.MethodGet, path: "/v1/plans/" + esc(planKey) + "/draft"}, &out, opts)
	return result(&out, err)
}

// PutDraft creates or fully replaces the plan's draft. values maps feature
// keys to values; unlisted features are omitted from the draft.
func (c *Client) PutDraft(ctx context.Context, planKey string, values map[string]Value, opts ...RequestOption) (*PlanVersion, error) {
	if values == nil {
		values = map[string]Value{}
	}
	var out PlanVersion
	_, err := c.do(ctx, request{method: http.MethodPut, path: "/v1/plans/" + esc(planKey) + "/draft",
		body: PutDraftRequest{Values: values}, mutation: true}, &out, opts)
	return result(&out, err)
}

// PublishPlan publishes the draft as the next immutable version. It returns
// ErrNoDraft when there is nothing to publish.
func (c *Client) PublishPlan(ctx context.Context, planKey string, opts ...RequestOption) (*PlanVersion, error) {
	var out PlanVersion
	_, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/plans/" + esc(planKey) + "/publish", mutation: true}, &out, opts)
	return result(&out, err)
}

// ListPlanVersions returns one page of published versions, without values.
func (c *Client) ListPlanVersions(ctx context.Context, planKey string, page *ListOptions, opts ...RequestOption) (*PlanVersionList, error) {
	var out PlanVersionList
	_, err := c.do(ctx, request{method: http.MethodGet, path: "/v1/plans/" + esc(planKey) + "/versions", query: page.query()}, &out, opts)
	return result(&out, err)
}

// GetPlanVersion returns a published version with its values.
func (c *Client) GetPlanVersion(ctx context.Context, planKey string, version int, opts ...RequestOption) (*PlanVersion, error) {
	var out PlanVersion
	_, err := c.do(ctx, request{method: http.MethodGet,
		path: "/v1/plans/" + esc(planKey) + "/versions/" + strconv.Itoa(version)}, &out, opts)
	return result(&out, err)
}

// ------------------------------------------------- customers and subscriptions

// UpsertCustomer registers a customer by your own opaque ID (not an email or
// name). It is idempotent; created reports whether a new record was made.
func (c *Client) UpsertCustomer(ctx context.Context, customerID string, opts ...RequestOption) (cust *Customer, created bool, err error) {
	var out Customer
	res, err := c.do(ctx, request{method: http.MethodPut, path: "/v1/customers/" + esc(customerID), mutation: true}, &out, opts)
	if err != nil {
		return nil, false, err
	}
	return &out, res.status == http.StatusCreated, nil
}

// GetCustomer returns a customer by external ID.
func (c *Client) GetCustomer(ctx context.Context, customerID string, opts ...RequestOption) (*Customer, error) {
	var out Customer
	_, err := c.do(ctx, request{method: http.MethodGet, path: "/v1/customers/" + esc(customerID)}, &out, opts)
	return result(&out, err)
}

// AssignSubscription gives the customer a new current subscription,
// replacing any existing one. Leave Plan.Version nil to pin the latest
// published version.
func (c *Client) AssignSubscription(ctx context.Context, customerID string, in AssignSubscriptionRequest, opts ...RequestOption) (*Subscription, error) {
	var out Subscription
	_, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/customers/" + esc(customerID) + "/subscriptions",
		body: in, mutation: true}, &out, opts)
	return result(&out, err)
}

// GetCurrentSubscription returns the customer's current subscription.
func (c *Client) GetCurrentSubscription(ctx context.Context, customerID string, opts ...RequestOption) (*Subscription, error) {
	var out Subscription
	_, err := c.do(ctx, request{method: http.MethodGet, path: "/v1/customers/" + esc(customerID) + "/subscriptions/current"}, &out, opts)
	return result(&out, err)
}

// SubscriptionPatch changes status and lifecycle timestamps. Unset
// fields are unchanged; Null clears a timestamp. Any status change is
// allowed if the result is a valid subscription.
type SubscriptionPatch struct {
	Status             *SubscriptionStatus
	CurrentPeriodStart Patch[time.Time]
	CurrentPeriodEnd   Patch[time.Time]
	TrialEndsAt        Patch[time.Time]
	GraceEndsAt        Patch[time.Time]
	CancelAt           Patch[time.Time]
}

// MarshalJSON writes only the fields that are set.
func (p SubscriptionPatch) MarshalJSON() ([]byte, error) {
	m := patchBody{}
	if p.Status != nil {
		m["status"] = *p.Status
	}
	m.add("current_period_start", p.CurrentPeriodStart)
	m.add("current_period_end", p.CurrentPeriodEnd)
	m.add("trial_ends_at", p.TrialEndsAt)
	m.add("grace_ends_at", p.GraceEndsAt)
	m.add("cancel_at", p.CancelAt)
	return m.marshal()
}

// UpdateSubscription changes the current subscription's lifecycle. It
// returns ErrSubscriptionReplaced for a subscription that is no longer current.
func (c *Client) UpdateSubscription(ctx context.Context, subscriptionID string, in SubscriptionPatch, opts ...RequestOption) (*Subscription, error) {
	var out Subscription
	_, err := c.do(ctx, request{method: http.MethodPatch, path: "/v1/subscriptions/" + esc(subscriptionID),
		body: in, mutation: true}, &out, opts)
	return result(&out, err)
}

// ChangePlanVersion moves the current subscription to another published
// version, keeping its lifecycle. Leave plan.Version nil for the latest.
func (c *Client) ChangePlanVersion(ctx context.Context, subscriptionID string, plan PlanSelector, opts ...RequestOption) (*Subscription, error) {
	var out Subscription
	_, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/subscriptions/" + esc(subscriptionID) + "/change-plan-version",
		body: ChangePlanVersionRequest{Plan: plan}, mutation: true}, &out, opts)
	return result(&out, err)
}

// ----------------------------------------------------------------- overrides

// CreateOverride adds a customer-specific value. It returns
// ErrOverrideOverlap if it overlaps an existing override for the feature.
func (c *Client) CreateOverride(ctx context.Context, customerID string, in CreateOverrideRequest, opts ...RequestOption) (*Override, error) {
	var out Override
	_, err := c.do(ctx, request{method: http.MethodPost, path: "/v1/customers/" + esc(customerID) + "/overrides",
		body: in, mutation: true}, &out, opts)
	return result(&out, err)
}

// ListOverrides returns one page of a customer's overrides, revoked ones
// included, ordered by creation.
func (c *Client) ListOverrides(ctx context.Context, customerID string, page *ListOptions, opts ...RequestOption) (*OverrideList, error) {
	var out OverrideList
	_, err := c.do(ctx, request{method: http.MethodGet, path: "/v1/customers/" + esc(customerID) + "/overrides", query: page.query()}, &out, opts)
	return result(&out, err)
}

// RevokeOverride revokes an override, keeping its history. Revoking an
// already-revoked override returns it unchanged.
func (c *Client) RevokeOverride(ctx context.Context, overrideID string, opts ...RequestOption) (*Override, error) {
	var out Override
	_, err := c.do(ctx, request{method: http.MethodDelete, path: "/v1/overrides/" + esc(overrideID), mutation: true}, &out, opts)
	return result(&out, err)
}

func result[T any](out *T, err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	return out, nil
}
