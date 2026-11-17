// Command quickstart evaluates one feature, then every feature, for one
// Customer with the Seal0 Go SDK.
//
//	export SEAL0_API_KEY=<API_KEY>   # an evaluate-scope key from the Console
//	go run ./examples/quickstart cus_test_0001 max_projects
//
// It prints whether the feature is granted, its value, why, and until when
// the result can be cached (valid_until).
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	seal0 "github.com/seal0-app/sdk-go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "quickstart:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 3 {
		return errors.New("usage: quickstart <customer_id> <feature_key>")
	}
	customerID, featureKey := os.Args[1], os.Args[2]

	opts := []seal0.Option{}
	if u := os.Getenv("SEAL0_BASE_URL"); u != "" {
		opts = append(opts, seal0.WithBaseURL(u)) // defaults to https://api.seal0.app
	}
	client, err := seal0.New(os.Getenv("SEAL0_API_KEY"), opts...)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ev, err := client.Evaluate(ctx, customerID, featureKey)
	switch {
	case errors.Is(err, seal0.ErrCustomerNotFound):
		return fmt.Errorf("no Customer %q in this environment: create one in the Console first", customerID)
	case errors.Is(err, seal0.ErrFeatureNotFound):
		return fmt.Errorf("no feature %q in this environment", featureKey)
	case err != nil:
		return err // in an application, fail closed: treat it as not granted
	}
	printEntitlement(ev.Entitlement)

	all, err := client.EvaluateAll(ctx, customerID)
	if err != nil {
		return err
	}
	fmt.Printf("\nAll %d features for %s:\n", len(all.Entitlements), customerID)
	for _, e := range all.Entitlements {
		printEntitlement(e)
	}
	return nil
}

func printEntitlement(e seal0.Entitlement) {
	value := "none"
	if e.Value != nil {
		value = e.Value.String()
	}
	until := "no known change"
	if e.ValidUntil != nil {
		until = e.ValidUntil.Format(time.RFC3339)
	}
	fmt.Printf("%-24s granted=%-5t value=%-10s reason=%s valid_until=%s\n", e.FeatureKey, e.Granted, value, e.Reason, until)
}
