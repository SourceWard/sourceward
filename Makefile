.PHONY: build test test-e2e test-distribution test-acceptance fmt fmt-check vet check clean

build:
	mkdir -p bin
	go build -o bin/sourceward ./cmd/sourceward

test:
	go test ./...

test-e2e:
	./scripts/test-e2e.sh

test-distribution:
	./scripts/test-distribution.sh

test-acceptance:
	go test -count=1 ./internal/acceptance

fmt:
	gofmt -w ./cmd ./internal

fmt-check:
	test -z "$$(gofmt -l .)"

vet:
	go vet ./...

check: fmt-check vet test test-e2e test-distribution test-acceptance build

clean:
	rm -f bin/sourceward
