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
		PATH="$(GOPATH)/bin:$$PATH" protoc --proto_path="$$DIR" -I ./third_party -I ./boot -I . --go_out=paths=source_relative:"$$DIR" "$$PROTO_FILE"; \
	done

MODULES_VERSION ?=
MODULES ?=
ACTIVE_CORE_FMT_FILES = $$(find . -maxdepth 1 \( -name 'app*.go' -o -name 'runtime*.go' -o -name 'recovery*.go' -o -name 'circuit_breaker*.go' \)) \
	$$(find ./boot \( -name 'application*.go' -o -name 'configuration.go' \)) \
	$$(find ./events \( -name 'global.go' -o -name 'listeners.go' -o -name 'lynx_event_bus*.go' \)) \
	$$(find ./plugins -name 'unified_runtime*.go')
REPO_FMT_FILES = $$(find . -name '*.go' -not -path './third_party/*')

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

.PHONY: fmt
# format core Go files in root, boot, events, and plugins
fmt:
	gofmt -w $$(find . -maxdepth 1 -name '*.go') $$(find ./boot ./events ./plugins -name '*.go')

.PHONY: fmt-check
# verify formatting for core Go files in root, boot, events, and plugins
fmt-check:
	@out=$$(gofmt -l $$(find . -maxdepth 1 -name '*.go') $$(find ./boot ./events ./plugins -name '*.go')); \
	if [ -n "$$out" ]; then \
		echo "The following files need gofmt:"; \
		echo "$$out"; \
		exit 1; \
	fi

.PHONY: test-core
# run core package regression tests
test-core:
	go test . ./boot ./events ./plugins

.PHONY: test-compat
# run compatibility-surface regression tests
test-compat:
	go test . ./boot ./events ./plugins -run 'Test(Runtime|App|Application_|NewApplication_|FormatStartupElapsed|GetGlobalEventBus_UsesSharedFallbackAfterInitFailure|GlobalListenerManager_UsesProviderSafely|ExplicitManagerHelpers_DoNotRequireGlobals|PrivateResourceNamespace_DoesNotCollideWithShared|PrivateResourceInfo_ResolvesLegacyDisplayNameWithoutBreakingSharedStorage|CircuitBreaker_|ErrorRecoveryManager_)'

.PHONY: test-extended
# run broader supporting-package regression tests outside the active core surface
test-extended:
	go test ./cache ./subscribe ./tls ./observability/... ./pkg/... ./log

.PHONY: validate
# verify formatting plus core, compatibility, and broader supporting-package regressions
validate: fmt-check test-core test-compat test-extended

.PHONY: fmt-repo
# format all repository Go files outside vendored third_party code
fmt-repo:
	gofmt -w $(REPO_FMT_FILES)

.PHONY: fmt-repo-check
# verify formatting for all repository Go files outside vendored third_party code
fmt-repo-check:
	@out=$$(gofmt -l $(REPO_FMT_FILES)); \
	if [ -n "$$out" ]; then \
		echo "The following repository files need gofmt:"; \
		echo "$$out"; \
		exit 1; \
	fi

.PHONY: race-core
# run race detection for core packages when the current environment supports ThreadSanitizer
race-core:
	go test -race . ./boot ./events ./plugins

.PHONY: fmt-active
# format the actively refactored core surfaces
fmt-active:
	gofmt -w $(ACTIVE_CORE_FMT_FILES)

.PHONY: fmt-active-check
# verify formatting for the actively refactored core surfaces
fmt-active-check:
	@out=$$(gofmt -l $(ACTIVE_CORE_FMT_FILES)); \
	if [ -n "$$out" ]; then \
		echo "The following active-core files need gofmt:"; \
		echo "$$out"; \
		exit 1; \
	fi

.PHONY: validate-active
# verify formatting plus compatibility regressions for the actively refactored core surfaces
validate-active: fmt-active-check test-compat

.PHONY: validate-broad
# verify active-core regressions plus broader supporting-package regressions
validate-broad: validate-active test-extended

.PHONY: validate-repo
# verify repository formatting plus full package test coverage
validate-repo: fmt-repo-check
	go test ./...

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
