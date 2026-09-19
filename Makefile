GO ?= go

.PHONY: all check build fmt fmt-check vet lint test tidy clean

all: check

check: fmt-check vet lint test build

build:
	$(GO) build ./...

fmt:
	$(GO) fmt ./...

fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needs to be run on:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

lint:
	golangci-lint run ./...

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

clean:
	$(GO) clean ./...
