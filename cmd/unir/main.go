package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/seemethere/unir/internal"
	log "github.com/sirupsen/logrus"
)

var (
	WEBHOOK_SECRET = "UNIR_WEBHOOK_SECRET"
	INTEGRATION_ID = "UNIR_INTEGRATION_ID"
	KEYFILE        = "UNIR_KEYFILE"
)

func main() {
	id := (os.Getenv(INTEGRATION_ID))

	debug := flag.Bool("debug", false, "Toggle debug mode")
	oauth := flag.Bool("oauth_mode", false, "Toggle for oauth")
	port := flag.String("port", "8080", "Port to bind unir to")
	flag.Parse()

	// Use either a test handler which allows you to use a webhook
	// or the regular handler which is integrated with an app
	var router *mux.Router
	if *oauth {
		router = internal.GenerateTestWebhookRouter(
			[]byte(os.Getenv(WEBHOOK_SECRET)),
			id,
			os.Getenv(KEYFILE),
		)
	} else {
		id, err := strconv.Atoi(id)
		if err != nil {
			log.Fatal(err)
		}

		router = internal.NewWebhookHandler(
			[]byte(os.Getenv(WEBHOOK_SECRET)),
			id,
			os.Getenv(KEYFILE),
		)
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Log level set to debug")
	}
	log.Infof("Starting unir on port %s", *port)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", *port), router))
}
