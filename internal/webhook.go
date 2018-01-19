package internal

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type GithubWebhookHandler struct {
	Secret        []byte
	integrationID int
	keyfile       string
}

func NewWebhookHandler(secret []byte, integrationID int, keyfile string) *mux.Router {
	handler := GithubWebhookHandler{
		Secret:        secret,
		integrationID: integrationID,
		keyfile:       keyfile,
	}
	router := mux.NewRouter()
	router.Handle("/", http.HandlerFunc(handler.handleGithubWebhook)).Methods("POST")
	return router
}

func (handler *GithubWebhookHandler) handleGithubWebhook(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Recieved POST request")
	payload, err := github.ValidatePayload(r, handler.Secret)
	if err != nil {
		log.Errorf("Failed to validate webhook secret from %s, %v", r.RemoteAddr, err)
		http.Error(w, "Secret did not match", http.StatusUnauthorized)
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Errorf("Failed to parse webhook from %s, %v", r.RemoteAddr, err)
		http.Error(w, "Bad payload", http.StatusBadRequest)
		return
	}
	switch e := event.(type) {
	case *github.PullRequestReviewEvent:
		go handlePullRequestReview(handler.integrationID, handler.keyfile, *e)
	case *github.StatusEvent:
		log.Infof("Recieved a status event")
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

func getPullRequestFiles(ctx context.Context, client *github.Client, e github.PullRequestReviewEvent) ([]*github.CommitFile, error) {
	log.Debugf("[%s] Pull pull request files", *e.Review.HTMLURL)
	opt := &github.ListOptions{}
	var files []*github.CommitFile
	for {
		filesByPage, resp, err := client.PullRequests.ListFiles(
			ctx,
			*e.Repo.Owner.Login,
			*e.Repo.Name,
			*e.PullRequest.Number,
			opt,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, filesByPage...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return files, nil
}

func editingConfig(ctx context.Context, client *github.Client, e github.PullRequestReviewEvent) bool {
	changedFiles, err := getPullRequestFiles(ctx, client, e)
	if err != nil {
		log.Errorf("[%s] Error grabbing changed files: %v", *e.Review.HTMLURL, err)
	}
	for _, file := range changedFiles {
		if file.GetFilename() == ".unir.yml" {
			log.Errorf("[%s] .unir.yml found in PR, skipping", *e.Review.HTMLURL)
			return true
		}
	}
	return false
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
			reviewMap[strings.ToLower(*review.User.Login)] = true
		case "CHANGES_REQUESTED":
			reviewMap[strings.ToLower(*review.User.Login)] = false
		}
	}
	return reviewMap
}

func GrabConfig(ctx context.Context, client *github.Client, repo, owner string, baseRef string) (UnirConfig, error) {
	r, err := client.Repositories.DownloadContents(
		ctx,
		owner,
		repo,
		".unir.yml",
		&github.RepositoryContentGetOptions{Ref: baseRef},
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

// TODO: Write a function to save from people editing `.unir.yml` and auto-merging it
//       You can do this by getting a list of files, checking if it contains `.unir.yml`
//       and exiting early before it gets to the point of potential merging

func handlePullRequestReview(integrationID int, keyfile string, e github.PullRequestReviewEvent) {
	itr, err := ghinstallation.NewKeyFromFile(
		http.DefaultTransport,
		integrationID,
		*e.Installation.ID,
		keyfile,
	)
	if err != nil {
		log.Fatal(err)
	}
	client := github.NewClient(&http.Client{Transport: itr})
	log.Debugf("[%s] STARTED handling pull request review", *e.Review.HTMLURL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	allReviews, err := getPullRequestReviews(ctx, client, e)
	if err != nil {
		log.Errorf("[%s] Error grabbing pull request reviews: %v", *e.Review.HTMLURL, err)
		return
	}
	reviews := RemoveStaleReviews(*e.PullRequest.Head.SHA, allReviews)
	config, err := GrabConfig(ctx, client, *e.Repo.Name, *e.Repo.Owner.Login, e.PullRequest.Base.GetRef())
	if err != nil {
		log.Errorf("[%s] Error grabbing configuration: %v", *e.Review.HTMLURL, err)
		return
	}
	votes := GenerateReviewMap(reviews)
	opts := AgreementOptions{
		NeedsConsensus: config.ConsensusNeeded,
		Threshold:      config.ApprovalsNeeded,
	}

	mergeMethod := "merge"
	if config.MergeMethod != "" {
		mergeMethod = config.MergeMethod
	}

	// Exit early on non-agreements
	if !AgreementReached(config.Whitelist, votes, &opts) {
		log.Infof("[%s] Agreement not reached! Staying put on %s/%s#%d", *e.Review.HTMLURL, *e.Repo.Owner.Login, *e.Repo.Name, *e.PullRequest.Number)
		return
	}

	if editingConfig(ctx, client, e) {
		return
	}

	log.Infof("[%s] Agreement reached! Merging %s/%s#%d", *e.Review.HTMLURL, *e.Repo.Owner.Login, *e.Repo.Name, *e.PullRequest.Number)
	result, resp, err := client.PullRequests.Merge(
		ctx,
		*e.Repo.Owner.Login,
		*e.Repo.Name,
		*e.PullRequest.Number,
		"Merged with https://github.com/seemethere/unir",
		&github.PullRequestOptions{MergeMethod: mergeMethod, SHA: *e.PullRequest.Head.SHA},
	)

	// We don't reach our success criteria
	if resp.StatusCode != 200 {
		log.Errorf("[%s] Merge failed for %s/%s#%d: %s, %v", *e.Review.HTMLURL, *e.Repo.Owner.Login, *e.Repo.Name, *e.PullRequest.Number, result.GetMessage(), err)
		errorMessage := fmt.Sprintf("Unable to merge! %s", result.GetMessage())
		_, _, err := client.Issues.CreateComment(
			ctx,
			*e.Repo.Owner.Login,
			*e.Repo.Name,
			*e.PullRequest.Number,
			&github.IssueComment{Body: &errorMessage},
		)
		if err != nil {
			log.Errorf(
				"[%s] Error posting comment in %s/%s#%d: %v",
				*e.Review.HTMLURL,
				*e.Repo.Owner.Login,
				*e.Repo.Name,
				*e.PullRequest.Number,
				err,
			)
		}
		return
	}

	log.Infof("[%s] Merge successful for %s/%s#%d", *e.Review.HTMLURL, *e.Repo.Owner.Login, *e.Repo.Name, *e.PullRequest.Number)
}
