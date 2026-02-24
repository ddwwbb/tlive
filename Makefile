.PHONY: build run test clean

BINARY=tlive

build:
	go build -o $(BINARY) ./cmd/tlive

run: build
	./$(BINARY)

test:
	go test ./... -v

clean:
	rm -f $(BINARY)
