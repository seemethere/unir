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
		go handleStatus(handler.integrationID, handler.keyfile, *e)
	}
}

func getPullRequestReviews(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	prNumber int,
) ([]*github.PullRequestReview, error) {
	log.Debugf("Pulling pull request reviews for https://github.com/%s/%s/pull/%d", owner, repo, prNumber)
	opt := &github.ListOptions{}
	var reviews []*github.PullRequestReview
	for {
		reviewsByPage, resp, err := client.PullRequests.ListReviews(
			ctx,
			owner,
			repo,
			prNumber,
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

func getPullRequestFiles(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	prNumber int,
) ([]*github.CommitFile, error) {
	log.Debugf("Pulling pull request files for https://github.com/%s/%s/pull/%d", owner, repo, prNumber)
	opt := &github.ListOptions{}
	var files []*github.CommitFile
	for {
		filesByPage, resp, err := client.PullRequests.ListFiles(
			ctx,
			owner,
			repo,
			prNumber,
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

func editingConfig(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	prNumber int,
) bool {
	changedFiles, err := getPullRequestFiles(ctx, client, owner, repo, prNumber)
	if err != nil {
		log.Errorf("Error grabbing changed files for https://github.com/%s/%s/pull/%d", owner, repo, prNumber)
	}
	for _, file := range changedFiles {
		if file.GetFilename() == ".unir.yml" {
			log.Errorf(".unir.yml found in PR https://github.com/%s/%s/pull/%d, skipping", owner, repo, prNumber)
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

func createGithubClient(integrationID, installationID int, keyfile string) *github.Client {
	itr, err := ghinstallation.NewKeyFromFile(
		http.DefaultTransport,
		integrationID,
		installationID,
		keyfile,
	)
	if err != nil {
		log.Fatal(err)
	}
	return github.NewClient(&http.Client{Transport: itr})
}

func handleStatus(integrationID int, keyfile string, statusEvent github.StatusEvent) {
	if *statusEvent.State != "success" {
		log.Debugf("Skipping unsuccessful commit status event %s", *statusEvent.TargetURL)
		return
	}
	client := createGithubClient(integrationID, *statusEvent.Installation.ID, keyfile)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	// Grab open pull requests relating to sha with the most updated being first
	// Could potentially not grab all pull requests if there are more than 100
	// pull requests that all reference the same commit SHA. ¯\_(ツ)_/¯
	query := fmt.Sprintf(
		"is:pr is:open sort:updated-desc sha:%s repo:%s/%s",
		*statusEvent.Commit.SHA,
		*statusEvent.Repo.Owner.Login,
		*statusEvent.Repo.Name,
	)
	searchResults, _, err := client.Search.Issues(ctx, query, nil)
	if err != nil {
		log.Errorf("Error grabbing issues related to SHA: %s", *statusEvent.Commit.SHA)
		return
	}
	for _, issue := range searchResults.Issues {
		pullRequest, _, err := client.PullRequests.Get(
			ctx,
			*statusEvent.Repo.Owner.Login,
			*statusEvent.Repo.Name,
			*issue.Number,
		)
		if err != nil {
			log.Errorf("Error grabbing pull request information for %s", issue.HTMLURL)
			return
		}
		// Don't waste API calls on old commits
		if *statusEvent.Commit.SHA != *pullRequest.Head.SHA {
			log.Infof("Commit status for commit %s does not match the head SHA of its associated PR, skipping...", *statusEvent.Commit.SHA)
			return
		}
		mergePullRequest(
			client,
			*statusEvent.Repo.Owner.Login,
			*statusEvent.Repo.Name,
			*pullRequest.Head.SHA,
			*pullRequest.Number,
		)
	}
}

func handlePullRequestReview(integrationID int, keyfile string, reviewEvent github.PullRequestReviewEvent) {
	client := createGithubClient(integrationID, *reviewEvent.Installation.ID, keyfile)
	log.Debugf("[%s] STARTED handling pull request review", *reviewEvent.Review.HTMLURL)
	mergePullRequest(client, *reviewEvent.Repo.Owner.Login, *reviewEvent.Repo.Name, *reviewEvent.PullRequest.Head.SHA, *reviewEvent.PullRequest.Number)
}

func mergePullRequest(client *github.Client, owner, repo, sha string, prNumber int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	githubURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, prNumber)
	allReviews, err := getPullRequestReviews(ctx, client, owner, repo, prNumber)
	if err != nil {
		log.Errorf("Error grabbing pull request reviews for %s: %v", githubURL, err)
		return
	}
	reviews := RemoveStaleReviews(sha, allReviews)
	config, err := GrabConfig(ctx, client, repo, owner, "master")
	if err != nil {
		log.Errorf("Error grabbing configuration for %s: %v", githubURL, err)
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
		log.Infof("Agreement not reached! Staying put on %s", githubURL)
		return
	}

	if editingConfig(ctx, client, owner, repo, prNumber) {
		return
	}

	log.Infof("Agreement reached! Merging %s", githubURL)
	result, resp, err := client.PullRequests.Merge(
		ctx,
		owner,
		repo,
		prNumber,
		"Merged with https://github.com/seemethere/unir",
		&github.PullRequestOptions{MergeMethod: mergeMethod, SHA: sha},
	)

	// We don't reach our success criteria
	if resp.StatusCode != 200 {
		log.Errorf("Merge failed for %s: %s, %v", githubURL, result.GetMessage(), err)
		errorMessage := fmt.Sprintf("Unable to merge! %s", result.GetMessage())
		_, _, err := client.Issues.CreateComment(
			ctx,
			owner,
			repo,
			prNumber,
			&github.IssueComment{Body: &errorMessage},
		)
		if err != nil {
			log.Errorf("Error posting comment in %s: %v", githubURL, err)
		}
		return
	}

	log.Infof("Merge successful for %s", githubURL)
}
