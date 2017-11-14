DEP="$(shell go env GOPATH)/bin/dep"

$(DEP):
	go get github.com/golang/dep/cmd/dep

.PHONY: clean
clean:
	rm -rf build

build/unir: cmd/unir/main.go
	mkdir -p build
	go build -v -o $@ $<

build: build/unir

.PHONY: test
test:
	go test -v ./...

.PHONY: ensure
ensure: $(DEP)
	$< ensure -v -update

.PHONY: image
image:
	docker build -t seemethere/unir:dev .

.PHONY: run-dev
run-dev: image
	docker run --rm -i -p 8080:8080 --name unir-dev seemethere/unir:dev
