DEP="$(shell go env GOPATH)/bin/dep"

$(DEP): ## Grab golang/dep utility
	go get github.com/golang/dep/cmd/dep

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: clean
clean: ## Clean the workspace
	rm -rf build

build/unir: ## Build the main application
build/unir: cmd/unir/main.go
	mkdir -p build
	go build -v -o $@ $<

build: ## Build all targets
build: build/unir

.PHONY: test
test: ## Run all tests
	go test -v ./...

.PHONY: ensure
ensure: ## Reload dependencies
ensure: $(DEP)
	$< ensure -v -update

.PHONY: image
image: ## Make docker image
	docker build -t seemethere/unir:dev .

.PHONY: run-dev
run-dev: ## Run server in development mode
run-dev: image
	docker run --rm -i -p 8080:8080 --name unir-dev -e UNIR_WEBHOOK_SECRET -e UNIR_CLIENT_TOKEN seemethere/unir:dev -debug

.PHONY: release
release: # Release images to Docker Hub
release: VERSION
	VERSION=$(shell cat $<) ./release.sh
	$(RM) CHANGELOG.md
	$(MAKE) CHANGELOG.md

CHANGELOG.md:
	docker run --rm \
		--interactive \
		--tty \
		--net "host" \
		-v "$(CURDIR):$(CURDIR)" \
		-w $(CURDIR) \
		-it muccg/github-changelog-generator -u seemethere -p unir
