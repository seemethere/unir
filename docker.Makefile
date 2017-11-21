GOLANG_IMAGE=golang:1.9.2
WORKDIR=/go/src/github.com/seemethere/unir
DOCKER_RUN=docker run --rm -i -v "$(CURDIR)":$(WORKDIR) -w $(WORKDIR)

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

shell: ## Run a shell in a docker image with code volume mounted
	$(DOCKER_RUN) -t $(GOLANG_IMAGE) sh

%:
	$(DOCKER_RUN) $(GOLANG_IMAGE) make $@
