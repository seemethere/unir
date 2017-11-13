DEP="$(shell go env GOPATH)/bin/dep"

$(DEP):
	go get github.com/golang/dep/cmd/dep

clean:
	rm -rf build

build/unir: cmd/unir/main.go
	mkdir -p build
	go build -v -o $@ $<

build: build/unir

test:
	go test -v ./...

ensure: $(DEP)
	$< ensure -v -update
