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
	"golang.org/x/oauth2"
)

type GithubWebhookHandler struct {
	Secret           []byte
	integrationID    int
	apiToken         string
	keyfile          string
	needsOAuthClient bool
}

func GenerateTestWebhookRouter(secret []byte, apiToken, keyfile string) *mux.Router {
	router := mux.NewRouter()
	handler := GithubWebhookHandler{
		Secret:           secret,
		apiToken:         apiToken,
		keyfile:          keyfile,
		needsOAuthClient: true,
	}
	router.Handle("/", http.HandlerFunc(handler.handleGithubWebhook)).Methods("POST")
	return router
}

func NewWebhookHandler(secret []byte, integrationID int, keyfile string) *mux.Router {
	handler := GithubWebhookHandler{
		Secret:           secret,
		integrationID:    integrationID,
		keyfile:          keyfile,
		needsOAuthClient: false,
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
		go handlePullRequestReview(handler.integrationID, handler.keyfile, handler.apiToken, *e, handler.needsOAuthClient)
	case *github.StatusEvent:
		go handleStatus(handler.integrationID, handler.keyfile, handler.apiToken, *e, handler.needsOAuthClient)
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

func createGithubClient(integrationID, installationID int, keyfile, apiToken string, needsOAuthClient bool) *github.Client {
	if needsOAuthClient {
		log.Debug("Creating a client based on test")
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: apiToken},
		)
		return github.NewClient(oauth2.NewClient(ctx, ts))
	}

	log.Debug("Creating a client regularly")
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

func handleStatus(integrationID int, keyfile, apiToken string, statusEvent github.StatusEvent, needsOAuthClient bool) {
	// Exit early when handling our own statuses
	if statusEvent.GetContext() == "unir" {
		return
	}
	if *statusEvent.State != "success" {
		url := ""
		if statusEvent.TargetURL != nil {
			url = *statusEvent.TargetURL
		}
		log.Debugf("Skipping unsuccessful commit status event", url)
		return
	}

	installationID := 0
	if !needsOAuthClient {
		installationID = int(*statusEvent.Installation.ID)
	}

	client := createGithubClient(integrationID, installationID, keyfile, apiToken, needsOAuthClient)
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
			*pullRequest.Title,
		)
	}
}

func handlePullRequestReview(integrationID int, keyfile, apiToken string, reviewEvent github.PullRequestReviewEvent, needsOAuthClient bool) {
	//client := createGithubClient(integrationID, int(*reviewEvent.Installation.ID), keyfile, apiToken, needsOAuthClient)
	installationID := 0
	if !needsOAuthClient {
		installationID = int(*reviewEvent.Installation.ID)
	}
	client := createGithubClient(integrationID, installationID, keyfile, apiToken, needsOAuthClient)
	log.Debugf("[%s] STARTED handling pull request review", *reviewEvent.Review.HTMLURL)
	mergePullRequest(client, *reviewEvent.Repo.Owner.Login, *reviewEvent.Repo.Name, *reviewEvent.PullRequest.Head.SHA, *reviewEvent.PullRequest.Number, *reviewEvent.PullRequest.Title)
}

func checkMergeBlockKeywords(mergeBlockKeywords []string, prTitle string) bool {
	if len(mergeBlockKeywords) == 0 {
		mergeBlockKeywords = append(mergeBlockKeywords, "WIP:")
	}
	for _, word := range mergeBlockKeywords {
		if strings.Contains(prTitle, word) {
			return true
		}
	}
	return false
}

func mergePullRequest(client *github.Client, owner, repo, sha string, prNumber int, prTitle string) {
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

	doStatus := func(step, state, description string) {
		statusContext := "unir"
		// Create a commit status on the SHA that represents the merge status
		_, _, err = client.Repositories.CreateStatus(
			ctx,
			owner,
			repo,
			sha,
			&github.RepoStatus{
				State:       &state,
				Description: &description,
				Context:     &statusContext,
			},
		)

		if err != nil {
			log.Errorf("Creating commit status failed for %s on step %s: %v", githubURL, step, err)
		}
	}

	if editingConfig(ctx, client, owner, repo, prNumber) {
		doStatus("config editing", "failure", "Unable to merge automatically, editing unir config")
		return
	}

	if checkMergeBlockKeywords(config.MergeBlockKeywords, prTitle) {
		doStatus("checking keywords that block merging", "failure", "Automatic merging blocked, title contains keywords that prevent unir from automatically merging")
		return
	}

	reached, reason := AgreementReached(config.Whitelist, votes, &opts)
	// Exit early on non-agreements
	if !reached {
		doStatus("setting pending status", "pending", fmt.Sprintf("Automatic merge is pending, %s", reason))
		log.Infof("Agreement not reached! Staying put on %s", githubURL)
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
		doStatus("merge failure", "failure", "Failed to merge automatically")
		return
	}
	doStatus("successful merge", "success", "Merged automatically with unir")
	log.Infof("Merge successful for %s", githubURL)
}
