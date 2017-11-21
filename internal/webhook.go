package internal

import (
	"context"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type GithubWebhookHandler struct {
	Secret []byte
	Client *github.Client
}

func NewWebhookHandler(secret []byte, clientToken string) *mux.Router {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: clientToken},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))
	handler := GithubWebhookHandler{
		Secret: secret,
		Client: client,
	}
	router := mux.NewRouter()
	router.Handle("/{owner:.*}/{repo:.*}", http.HandlerFunc(handler.handleGithubWebhook)).Methods("POST")
	return router
}

func (handler *GithubWebhookHandler) handleGithubWebhook(w http.ResponseWriter, r *http.Request) {
	log.Debugf("[%s] Recieved webhook", r.RequestURI)
	payload, err := github.ValidatePayload(r, handler.Secret)
	if err != nil {
		log.Errorf("[%s] Failed to validate webhook secret, %v", r.RequestURI, err)
		http.Error(w, "Secret did not match", http.StatusUnauthorized)
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Errorf("[%s] Failed to parse webhook, %v", r.RequestURI, err)
		http.Error(w, "Bad payload", http.StatusBadRequest)
		return
	}
	switch e := event.(type) {
	case *github.PullRequestReviewEvent:
		go handlePullRequestReview(handler.Client, *e)
	}
}

func getPullRequestReviews(ctx context.Context, client *github.Client, e github.PullRequestReviewEvent) ([]*github.PullRequestReview, error) {
	log.Debugf("[%s] Pulling pull request reviews", *e.Review.HTMLURL)
	opt := &github.ListOptions{}
	var reviews []*github.PullRequestReview
	for {
		reviewsByPage, resp, err := client.PullRequests.ListReviews(
			ctx,
			*e.Repo.Owner.Login,
			*e.Repo.Name,
			*e.PullRequest.Number,
			opt,
		)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, reviewsByPage...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return reviews, nil
}

// TODO: Write a test
func RemoveStaleReviews(currentSha string, reviews []*github.PullRequestReview) []*github.PullRequestReview {
	var freshReviews []*github.PullRequestReview
	for _, review := range reviews {
		// Ignore stale reviews
		if *review.CommitID != currentSha {
			log.Debugf(
				"Skipping review %s, sha does not match latest",
				*review.HTMLURL,
			)
			continue
		}
		freshReviews = append(freshReviews, review)
	}
	return freshReviews
}

func GenerateReviewMap(reviews []*github.PullRequestReview) map[string]bool {
	reviewMap := make(map[string]bool)
	for _, review := range reviews {
		switch *review.State {
		// Cases outside of these 2 do not matter
		case "APPROVED":
			reviewMap[*review.User.Login] = true
			break
		case "CHANGES_REQUEST":
			reviewMap[*review.User.Login] = false
			break
		}
	}
	return reviewMap
}

func GrabConfig(ctx context.Context, client *github.Client, repo, owner string) (UnirConfig, error) {
	r, err := client.Repositories.DownloadContents(
		ctx,
		owner,
		repo,
		".unir.yml",
		&github.RepositoryContentGetOptions{Ref: "master"},
	)
	// TODO: Handle errors when this file doesn't exist
	if err != nil {
		return UnirConfig{}, err
	}
	body, err := ioutil.ReadAll(r)
	if err != nil {
		return UnirConfig{}, err
	}
	config, err := ReadConfig(body)
	if err != nil {
		return UnirConfig{}, err
	}
	return config, nil
}

func handlePullRequestReview(client *github.Client, e github.PullRequestReviewEvent) {
	log.Debugf("[%s] STARTED handling pull request review", *e.Review.HTMLURL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	allReviews, err := getPullRequestReviews(ctx, client, e)
	if err != nil {
		log.Errorf("[%s] Error grabbing pull request reviews: %v", *e.Review.HTMLURL, err)
		return
	}
	reviews := RemoveStaleReviews(*e.PullRequest.Head.SHA, allReviews)
	config, err := GrabConfig(ctx, client, *e.Repo.Name, *e.Repo.Owner.Login)
	if err != nil {
		log.Errorf("[%s] Error grabbing configuration: %v", *e.Review.HTMLURL, err)
		return
	}
	votes := GenerateReviewMap(reviews)
	opts := AgreementOptions{
		NeedsConsensus: config.ConsensusNeeded,
		Threshold:      config.ApprovalsNeeded,
	}
	if AgreementReached(config.Whitelist, votes, &opts) {
		log.Debugf("Agreement reached!")
	}
}
