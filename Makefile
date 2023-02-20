build:
	CGO_ENABLED=0 go build -ldflags '-s -w' -trimpath -o dist/sendEmail ./cmd/main.go

# go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
lint:
	golangci-lint run ./...