# 插件用 Makefile 模板：仅包含 proto 生成（config）
# 由 lynx/script/regenerate_protos.py 复制到各插件目录使用；勿在 lynx 主库/lynx-layout 覆盖已有 Makefile。

GOHOSTOS := $(shell go env GOHOSTOS)
GOPATH := $(shell go env GOPATH)
# 从插件目录（如 lynx-etcd）看，../lynx/third_party 指向主库的 google/protobuf 等
THIRD_PARTY ?= ../lynx/third_party

ifeq ($(GOHOSTOS), windows)
	Git_Bash := $(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	FIND_CMD := $(Git_Bash) -c "find . -name '*.proto'"
else
	FIND_CMD := find . -name '*.proto'
endif

.PHONY: init
# init env - 安装 proto 代码生成工具
init:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

.PHONY: config
# generate config - 根据当前目录下所有 .proto 重新生成 Go 代码
config:
	@for PROTO_FILE in $$($(FIND_CMD)); do \
		DIR=$$(dirname "$$PROTO_FILE"); \
		echo "  generating $$PROTO_FILE ..."; \
		PATH="$(GOPATH)/bin:$$PATH" protoc --proto_path="$$DIR" -I . -I "$(THIRD_PARTY)" --go_out=paths=source_relative:"$$DIR" "$$PROTO_FILE" || exit 1; \
	done

.PHONY: help
help:
	@echo ''
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@echo '  init    - 安装 protoc-gen-go / protoc-gen-go-grpc'
	@echo '  config  - 根据 .proto 重新生成 Go 代码'
	@echo ''

.DEFAULT_GOAL := help
