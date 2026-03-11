APP_NAME := dump-todos

.PHONY: build run run-incomplete build-macos clean

build:
	go build -o ./bin/$(APP_NAME) ./cmd/dump-todos

run:
	go run ./cmd/dump-todos

run-incomplete:
	go run ./cmd/dump-todos --incomplete

build-macos:
	./scripts/build-macos.sh

clean:
	rm -rf ./bin ./dist