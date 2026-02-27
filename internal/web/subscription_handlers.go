package web

import (
	"net/http"
)

// handleSubscriptions serves the self-service subscriptions page
func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	sub, err := s.services.Newsletter.GetOrCreateSubscriber(user.Email)
	if err != nil {
		s.renderError(w, r, "Failed to load subscriber", err)
		return
	}

	activeOnly := true
	repos, err := s.db.ListRepositories(&activeOnly)
	if err != nil {
		s.renderError(w, r, "Failed to load repositories", err)
		return
	}

	subscriptions, err := s.services.Newsletter.GetSubscriptions(sub.ID)
	if err != nil {
		s.renderError(w, r, "Failed to load subscriptions", err)
		return
	}

	subscribedRepos := make(map[int64]bool)
	for _, subscription := range subscriptions {
		subscribedRepos[subscription.RepoID] = true
	}

	repoViews := make([]SubscriptionRepoView, 0, len(repos))
	for _, repo := range repos {
		repoViews = append(repoViews, SubscriptionRepoView{
			ID:         repo.ID,
			Name:       repo.Name,
			Subscribed: subscribedRepos[repo.ID],
		})
	}

	data := PageData{
		Title:     "Subscriptions",
		ActiveNav: "subscriptions",
		User:      user,
		Content: SubscriptionsPageData{
			SubscribeAll: sub.SubscribeAll,
			Repos:        repoViews,
		},
	}

	s.render(w, s.templates.subscriptions, data)
}

// handleSubscriptionToggle handles subscribing/unsubscribing from a single repo
func (s *Server) handleSubscriptionToggle(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	repoName := r.FormValue("repo_name")
	action := r.FormValue("action")

	if repoName == "" || (action != "subscribe" && action != "unsubscribe") {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var err error
	if action == "subscribe" {
		err = s.services.Newsletter.Subscribe(user.Email, repoName)
	} else {
		err = s.services.Newsletter.Unsubscribe(user.Email, repoName)
	}

	if err != nil {
		s.renderError(w, r, "Failed to update subscription", err)
		return
	}

	http.Redirect(w, r, "/subscriptions", http.StatusSeeOther)
}

// handleSubscriptionToggleAll handles toggling the subscribe-to-all flag
func (s *Server) handleSubscriptionToggleAll(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	subscribeAll := r.FormValue("subscribe_all") == "on"

	if err := s.services.Newsletter.SetSubscribeAll(user.Email, subscribeAll); err != nil {
		s.renderError(w, r, "Failed to update subscription", err)
		return
	}

	http.Redirect(w, r, "/subscriptions", http.StatusSeeOther)
}
