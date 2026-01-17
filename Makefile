.PHONY: build clean stop start restart test

build:
	go build -o sh-server ./cmd/srv

clean:
	rm -f sh-server

test:
	go test ./...
