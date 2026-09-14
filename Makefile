.PHONY: build test fmt vet check clean

build:
	mkdir -p bin
	go build -o bin/sourceward ./cmd/sourceward

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

check: fmt vet test build

clean:
	rm -f bin/sourceward
