.PHONY: build test lint tidy check install-hooks

build:
	go build ./...

test:
	go test -race -cover ./...

lint:
	./scripts/lint.sh

tidy:
	go mod tidy

check:
	./scripts/check.sh

install-hooks:
	go tool lefthook install
