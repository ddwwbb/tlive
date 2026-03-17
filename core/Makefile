.PHONY: build run test clean release

BINARY=tlive-core
VERSION?=dev

build:
	go build -ldflags "-s -w" -o $(BINARY) ./cmd/tlive-core

run: build
	./$(BINARY)

test:
	go test ./... -v -timeout 30s

clean:
	rm -f $(BINARY) $(BINARY)-*

release:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BINARY)-linux-amd64 ./cmd/tlive-core
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BINARY)-darwin-amd64 ./cmd/tlive-core
	GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o $(BINARY)-darwin-arm64 ./cmd/tlive-core
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BINARY)-windows-amd64.exe ./cmd/tlive-core
