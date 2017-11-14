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

image:
	docker build -t seemethere/unir:dev .
