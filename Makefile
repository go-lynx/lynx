GOHOSTOS:=$(shell go env GOHOSTOS)
GOPATH:=$(shell go env GOPATH)
VERSION=$(shell git describe --tags --always)

ifeq ($(GOHOSTOS), windows)
	#the `find.exe` is different from `find` in bash/shell.
	#to see https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/find.
	#changed to use git-bash.exe to run find cli or other cli friendly, caused of every developer has a Git.
	#Git_Bash= $(subst cmd\,bin\bash.exe,$(dir $(shell where git)))
	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	CONF_PROTO_FILES=$(shell $(Git_Bash) -c "find conf -name *.proto")
else
	CONF_PROTO_FILES=$(shell find conf -name *.proto)
endif

.PHONY: init
# init env
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest


.PHONY: config
# generate config
config:
	for PROTO_FILE in $$(find . -name '*.proto'); do \
		DIR=$$(dirname "$$PROTO_FILE"); \
		protoc --proto_path="$$DIR" -I ./third_party -I ./boot -I ./app --go_out=paths=source_relative:"$$DIR" "$$PROTO_FILE"; \
	done

MODULES_VERSION ?=
MODULES ?=

.PHONY: tag
# tag modules with version
tag:
	@if [ -z "$(MODULES_VERSION)" ]; then \
		echo "❌ MODULES_VERSION is required. Usage: make tag MODULES_VERSION=v2.0.0 MODULES=\"plugins/xxx plugins/yyy\""; \
		exit 1; \
	fi
	@if [ -z "$(MODULES)" ]; then \
		echo "❌ MODULES is required. Usage: make tag MODULES_VERSION=v2.0.0 MODULES=\"plugins/xxx plugins/yyy\""; \
		exit 1; \
	fi
	@echo "Tagging modules with version $(MODULES_VERSION)..."
	@for module in $(MODULES); do \
		TAG="$$module/$(MODULES_VERSION)"; \
		echo "Creating tag $$TAG"; \
		git tag $$TAG || { echo "Failed to tag $$TAG"; exit 1; }; \
	done

.PHONY: push-tags
# push tags to origin
push-tags:
	@if [ -z "$(MODULES_VERSION)" ]; then \
		echo "❌ MODULES_VERSION is required. Usage: make push-tags MODULES_VERSION=v2.0.0 MODULES=\"plugins/xxx plugins/yyy\""; \
		exit 1; \
	fi
	@if [ -z "$(MODULES)" ]; then \
		echo "❌ MODULES is required. Usage: make push-tags MODULES_VERSION=v2.0.0 MODULES=\"plugins/xxx plugins/yyy\""; \
		exit 1; \
	fi
	@echo "Pushing tags to origin..."
	@for module in $(MODULES); do \
		TAG="$$module/$(MODULES_VERSION)"; \
		echo "Pushing tag $$TAG"; \
		git push origin $$TAG || { echo "Failed to push $$TAG"; exit 1; }; \
	done

.PHONY: release
# release modules with version
release: tag push-tags

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
