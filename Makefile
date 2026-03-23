.PHONY: build test test-python lint clean docker-build docker-clean

BINARY := airlock
VERSION := 0.1.0
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/airlock

test:
	go test -v -race -cover ./...

test-python:
	cd proxy/addon && pip install -q -r requirements-dev.txt && python3 -m pytest -v

test-all: test test-python

lint:
	golangci-lint run ./...

docker-build:
	docker build -t airlock-claude:latest -f container/Dockerfile container/
	docker build -t airlock-proxy:latest -f proxy/Dockerfile proxy/

docker-clean:
	docker rmi airlock-claude:latest airlock-proxy:latest 2>/dev/null || true

clean:
	rm -rf bin/
