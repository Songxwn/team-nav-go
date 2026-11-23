BINARY := teamnav
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build run test vet fmt docker docker-run clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/teamnav

run:
	go run ./cmd/teamnav -data ./data

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

docker:
	docker build --build-arg VERSION=$(VERSION) -t team-nav-go:$(VERSION) .

docker-run:
	docker run --rm -p 8080:8080 -v $(PWD)/data:/data team-nav-go:$(VERSION)

clean:
	rm -rf bin