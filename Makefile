.DELETE_ON_ERROR:

export GOMAXPROCS = $(shell test $$(uname) = 'Darwin' && sysctl -n hw.logicalcpu_max || lscpu -p | egrep -v '^\#' | wc -l)

ARGS  ?= -v
TESTS ?= ./... -cover
DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
PACKAGES = $(shell go list ./...)
SRC   := $(shell find . -name '*.go')

#all: deps format test
all: deps

deps:
	go build $(GOFLAGS)	
