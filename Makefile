.PHONY: build test clean prepare update

GO=CGO_ENABLED=0 go
GOFLAGS=-ldflags

MICROSERVICES=example/cmd/device-simple/device-simple example/cmd/device-modbus/device-modbus example/cmd/device-system/device-system
.PHONY: $(MICROSERVICES)

build: $(MICROSERVICES)
	go build ./...

example/cmd/device-simple/device-simple:
	$(GO) build -o $@ ./example/cmd/device-simple

example/cmd/device-modbus/device-modbus:
	$(GO) build -o $@ ./example/cmd/device-modbus

example/cmd/device-system/device-system:
	$(GO) build -o $@ ./example/cmd/device-system

test:
	go test ./... -cover

clean:
	rm -f $(MICROSERVICES)

prepare:
	glide install

update:
	glide update
