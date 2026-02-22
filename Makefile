BINARY := changelog-generator
GOBIN  := $(shell go env GOPATH)/bin

.PHONY: build install uninstall

build:
	go build -o $(BINARY) .

install:
	go build -o $(GOBIN)/$(BINARY) .

uninstall:
	rm -f $(GOBIN)/$(BINARY)
