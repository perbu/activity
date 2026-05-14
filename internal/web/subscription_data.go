package web

// SubscriptionsPageData is the view model for the self-service subscriptions page.
type SubscriptionsPageData struct {
	SubscribeAll bool
	Repos        []SubscriptionRepoView
}

// SubscriptionRepoView represents a single repo row in the subscriptions page.
type SubscriptionRepoView struct {
	ID         int64
	Name       string
	Subscribed bool
}
