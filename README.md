# Seal0 Go SDK

Go client for the Seal0 entitlement API. Standard library only (no
dependencies), Go 1.23+.

```sh
go get github.com/seal0-app/sdk-go
```

```go
import seal0 "github.com/seal0-app/sdk-go"

client, err := seal0.New(os.Getenv("SEAL0_API_KEY")) // s0_prod_… — keep it on the server

ev, err := client.Evaluate(ctx, "cus_123", "max_projects")
if err != nil {
	// Unknown customer/feature, auth or network failure: fail closed.
	return deny()
}
if n, unlimited, ok := ev.Entitlement.Value.Limit(); ok && (unlimited || projects < n) {
	return allow()
}
```

- **Typed errors:** `errors.Is(err, seal0.ErrCustomerNotFound)`; `*seal0.APIError` carries `Code`, `Field` and `RequestID`.
- **Metered usage:** `client.RecordUsage(ctx, "cus_123", "api_calls", 1)` adds to the current period (full-scope key); evaluation results of metered features carry `Entitlement.Usage` (used, limit, remaining, over limit, period). The API decides `Granted` from usage and the grace margin.
- **Safe retries:** network errors, 429 (except `usage_limit_exceeded`, which lasts until the usage period resets) and 5xx gateway errors are retried (2 by default). Every mutation sends an `Idempotency-Key`, reused across retries, so a retry never applies a change twice. Override it with `seal0.WithIdempotencyKey`.
- **PATCH:** `seal0.Set(v)` changes a field, `seal0.Null[T]()` clears it, and unset leaves it unchanged.
- **Pagination:** `for f, err := range client.Features(ctx) { … }`.
- **Values:** `seal0.BoolValue`, `NumberValue`, `UnlimitedValue`, `EnumValue`; read them with `Bool()`, `Limit()`, `Enum()`.

The SDK makes no entitlement decisions: the API does. Request and response
types are generated from the API contract (`types.gen.go`); do not edit them.
Use evaluate-scope keys in application servers and full-scope keys only where
you manage plans and customers.

## Getting started

1. Sign in to the [Seal0 Console](https://console.seal0.app) and create a project.
2. Create an **evaluate**-scope API key for your application servers (a **full**-scope key only where you manage plans and Customers). Keep keys on the server.
3. Define features and a plan, give a Customer a subscription, then evaluate it from your code.

The [documentation](https://docs.seal0.app) has the concepts (eligibility, precedence, `valid_until` caching), guides and the API reference.

## Example

[`examples/quickstart`](examples/quickstart/main.go) evaluates a feature for a Customer:

```sh
export SEAL0_API_KEY=<API_KEY>
go run ./examples/quickstart cus_test_0001 max_projects
```

## Versioning

Semantic versioning. Before 1.0.0, a minor version may include breaking changes; see [CHANGELOG.md](CHANGELOG.md).

## Contributing

This repository is published from Seal0's main repository on each release. Issues are welcome here. Pull requests are reviewed here and ported by hand, then arrive with the next release.

## License

[Apache-2.0](LICENSE).
