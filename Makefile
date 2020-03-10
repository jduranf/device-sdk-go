.PHONY: build arm test clean

GO=go

MICROSERVICES=example/cmd/device-simple/device-simple example/cmd/device-modbus/device-modbus example/cmd/device-system/device-system
.PHONY: $(MICROSERVICES)

VERSION=$(shell cat ./VERSION)

GOFLAGS=-ldflags "-X github.com/edgexfoundry/device-sdk-go.Version=$(VERSION)"

GIT_SHA=$(shell git rev-parse HEAD)

build: $(MICROSERVICES)

example/cmd/device-simple/device-simple:
	$(GO) build $(GOFLAGS) -o $@ ./example/cmd/device-simple

example/cmd/device-modbus/device-modbus:
	$(GO) build $(GOFLAGS) -o $@ ./example/cmd/device-modbus

example/cmd/device-system/device-system:
	$(GO) build $(GOFLAGS) -o $@ ./example/cmd/device-system

arm:
	GOOS=linux GOARCH=arm $(GO) build $(GOFLAGS) -o example/cmd/device-modbus/device-modbus ./example/cmd/device-modbus
	GOOS=linux GOARCH=arm $(GO) build $(GOFLAGS) -o example/cmd/device-system/device-system ./example/cmd/device-system

test:
	$(GO) test ./... -cover

clean:
	rm -f $(MICROSERVICES)
