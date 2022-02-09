all: build

.PHONY: build
build:
	go build -o bin/fake-device-plugin main.go
