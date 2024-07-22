ARCH ?= amd64
OS ?= linux
PROG_NAME := gtdd

ifneq ($(shell which podman 2>/dev/null),)
	CONTAINER_CLI := $(shell which podman)
	MOUNT_OPTIONS := :Z
else ifneq ($(shell which docker 2>/dev/null),)
	CONTAINER_CLI := $(shell which docker)
	MOUNT_OPTIONS :=
endif

.PHONY: all
all: $(PROG_NAME)


$(PROG_NAME): $(shell find internal/ -name *.go) $(shell find cmd/gtdd/ -name *.go)
	GOOS=$(OS) GOARCH=$(ARCH) go build -o $(PROG_NAME) ./cmd/gtdd

.PHONY: clean
clean:
	go clean -cache -testcache
	rm -rf $(PROG_NAME)

.PHONY: dep
dep:
	go mod download

ifdef CONTAINER_CLI
.PHONY: lint
lint:
	$(CONTAINER_CLI) run -t --rm -v $(PWD):/app$(MOUNT_OPTIONS) \
		-w /app \
		golangci/golangci-lint:latest \
		golangci-lint run
endif

ifneq (,$(wildcard $(GOPATH)/bin/godoc))
.PHONY: docs
docs:
	@$(GOPATH)/bin/godoc -http=:6060
endif


.PHONY: check
check:
	go test -covermode=atomic -race -coverprofile=coverage.cov ./...
	go tool cover -html=coverage.cov -o coverage.html


ifneq (,$(wildcard $(GOPATH)/bin/gofumpt))
.PHONY: format
format:
	@$(GOPATH)/bin/gofumpt -l -w .
else
.PHONY: format
format:
	@go fmt ./...
endif
