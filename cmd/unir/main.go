package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/seemethere/unir/internal"
	log "github.com/sirupsen/logrus"
)

var (
	WEBHOOK_SECRET = "UNIR_WEBHOOK_SECRET"
	CLIENT_TOKEN   = "UNIR_CLIENT_TOKEN"
)

func main() {
	router := internal.NewWebhookHandler([]byte(os.Getenv(WEBHOOK_SECRET)), os.Getenv(CLIENT_TOKEN))
	debug := flag.Bool("debug", false, "Toggle debug mode")
	port := flag.String("port", "8080", "Port to bind unir to")
	flag.Parse()
	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Log level set to debug")
	}
	log.Infof("Starting unir on port %s", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", *port), router))
}
