package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/seemethere/unir/internal"
	log "github.com/sirupsen/logrus"
)

var (
	WEBHOOK_SECRET = "UNIR_WEBHOOK_SECRET"
	INTEGRATION_ID = "UNIR_INTEGRATION_ID"
	KEYFILE        = "UNIR_KEYFILE"
)

func main() {
	id, err := strconv.Atoi(os.Getenv(INTEGRATION_ID))
	if err != nil {
		log.Fatal(err)
	}
	router := internal.NewWebhookHandler(
		[]byte(os.Getenv(WEBHOOK_SECRET)),
		id,
		os.Getenv(KEYFILE),
	)
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
