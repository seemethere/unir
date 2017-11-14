GOLANG_IMAGE=golang:1.9.2
WORKDIR=/go/src/github.com/seemethere/unir
DOCKER_RUN=docker run --rm -i -v "$(CURDIR)":$(WORKDIR) -w $(WORKDIR)

shell:
	$(DOCKER_RUN) -t $(GOLANG_IMAGE) sh

%:
	$(DOCKER_RUN) $(GOLANG_IMAGE) make $@
