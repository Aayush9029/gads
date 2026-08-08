.PHONY: build test check docs

build:
	go build -o gads .

test:
	go test -count=1 ./...

check:
	go test -count=1 ./...
	go test -race -count=1 ./...
	go vet ./...
	go mod verify
	go build -trimpath -ldflags "-s -w -X main.version=0.1.0-rc1" -o gads .

docs:
	go run ./cmd/gendocs
