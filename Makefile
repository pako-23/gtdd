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
all:
	GOOS=$(OS) GOARCH=$(ARCH) go build -o $(PROG_NAME)

.PHONY: clean
clean:
	go clean
	rm -rf $(PROG_NAME)

.PHONY: dep
dep:
	go mod download

ifdef CONTAINER_CLI
.PHONY: lint
lint:
	$(CONTAINER_CLI) run -t --rm -v $(PWD):/app$(MOUNT_OPTIONS) \
		-w /app \
		golangci/golangci-lint:v1.54.2 \
		golangci-lint run --enable-all
endif


ifneq (,$(wildcard $(GOPATH)/bin/godoc))
.PHONY: docs
docs:
	@$(GOPATH)/bin/godoc -http=:6060
endif
