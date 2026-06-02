.PHONY: all install bench test clean

all: test

test:
	go test ./...

install:
	go install ./...

bench:
	go test -bench=. -benchmem ./...

clean:
	go clean ./...
