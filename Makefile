.PHONY: clean build
clean:
	@go clean

build:
	@rm cmd/main
	@go build -o cmd/main ./...


