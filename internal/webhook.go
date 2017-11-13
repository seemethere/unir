package internal

import (
	"context"
	"net/http"

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
	event, err := github.ParseWebhook(github.WebHookType(r), payload)
	if err != nil {
		log.Errorf("[%s] Faile to parse webhook, %v", r.RequestURI, err)
		http.Error(w, "Bad payload", http.StatusBadRequest)
		return
	}
}
