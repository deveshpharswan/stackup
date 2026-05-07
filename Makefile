VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%d)
LDFLAGS  = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test e2e lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o stackup .

test:
	go test ./... -v

e2e:
	go test ./test/e2e/... -tags e2e -v -timeout 120s

lint:
	go vet ./...

clean:
	rm -f stackup
