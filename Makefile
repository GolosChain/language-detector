SHELL := /bin/bash
PWD    = $(shell pwd)

GOPATH=$(PWD)/go
GO=GOPATH=$(GOPATH) go
GODEBUG=GOPATH=$(GOPATH) PATH=$(GOPATH)/bin:$$PATH godebug


PKG  = . # $(dir $(wildcard ./*)) # uncomment for implicit submodules
BIN  = language-detector

FIND_STD_DEPS = $(GO) list std | sort | uniq
FIND_PKG_DEPS = $(GO) list -f '{{join .Deps "\n"}}' $(PKG) | sort | uniq | grep -v "^_"
DEPS          = $(shell comm -23 <($(FIND_PKG_DEPS)) <($(FIND_STD_DEPS)))
PORT 					= 3000

.PHONY: test

default: fmt deps test build

all: build
build: fmt 
	$(GO) build -a -o $(BIN) $(PKG)
lint: vet
vet: deps
	$(GO) get code.google.com/p/go.tools/cmd/vet
	$(GO) vet $(PKG)
fmt:
	$(GO) fmt $(PKG)
test:
	$(GO) test -a -v $(PKG)
cover: test-deps
	$(GO) test -cover $(PKG)
clean:
	$(GO) clean -i $(PKG)
clean-all:
	$(GO) clean -i -r $(PKG)
deps:
	curl -s https://raw.githubusercontent.com/bottlenose-inc/gpm/v1.3.6/bin/gpm > gpm.sh
	chmod 755 gpm.sh
	GOPATH=$(GOPATH) ./gpm.sh
	rm gpm.sh
test-deps: deps
	$(GO) get -d -t $(PKG)
	$(GO) test -i $(PKG)
install:
	$(GO) install
run: all
	./$(BIN)
curl:
	# Getting language from text:
	curl -H "Content-Type: application/json" -X POST -d '{"request": [{"text": "Starbucks coffee is really not good We have other stuff here too"}]}' localhost:$(PORT)
	@echo
	curl -H "Content-Type: application/json" -X POST -d '{"request": [{"text": "Starbucks coffee is really not good We have other stuff here too"}, {"text": "usa el tenedor para comer tu comida"}, {"text": "Je sais que les bleus ne vont pas gange le match"}]}' localhost:$(PORT)
	@echo
