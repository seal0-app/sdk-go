package seal0

import (
	"context"
	"iter"
)

// Iterators over every page of a list. Iteration stops at the first error,
// which is yielded with a zero value.
//
//	for f, err := range client.Features(ctx) {
//		if err != nil { return err }
//		fmt.Println(f.Key)
//	}

// Features iterates over all features, ordered by key.
func (c *Client) Features(ctx context.Context, opts ...RequestOption) iter.Seq2[Feature, error] {
	return paginate(func(page *ListOptions) ([]Feature, *string, error) {
		l, err := c.ListFeatures(ctx, page, opts...)
		if err != nil {
			return nil, nil, err
		}
		return l.Data, l.NextCursor, nil
	})
}

// Plans iterates over all plans, ordered by key.
func (c *Client) Plans(ctx context.Context, opts ...RequestOption) iter.Seq2[Plan, error] {
	return paginate(func(page *ListOptions) ([]Plan, *string, error) {
		l, err := c.ListPlans(ctx, page, opts...)
		if err != nil {
			return nil, nil, err
		}
		return l.Data, l.NextCursor, nil
	})
}

// PlanVersions iterates over a plan's published versions, oldest first.
func (c *Client) PlanVersions(ctx context.Context, planKey string, opts ...RequestOption) iter.Seq2[PlanVersionSummary, error] {
	return paginate(func(page *ListOptions) ([]PlanVersionSummary, *string, error) {
		l, err := c.ListPlanVersions(ctx, planKey, page, opts...)
		if err != nil {
			return nil, nil, err
		}
		return l.Data, l.NextCursor, nil
	})
}

// Overrides iterates over a customer's overrides, revoked ones included.
func (c *Client) Overrides(ctx context.Context, customerID string, opts ...RequestOption) iter.Seq2[Override, error] {
	return paginate(func(page *ListOptions) ([]Override, *string, error) {
		l, err := c.ListOverrides(ctx, customerID, page, opts...)
		if err != nil {
			return nil, nil, err
		}
		return l.Data, l.NextCursor, nil
	})
}

func paginate[T any](fetch func(*ListOptions) ([]T, *string, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		page := &ListOptions{Limit: 200}
		for {
			items, next, err := fetch(page)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if next == nil || *next == "" {
				return
			}
			page = &ListOptions{Limit: 200, Cursor: *next}
		}
	}
}
