run:
	go run .

run-help:
	go run . --help

test:
	go test -coverprofile coverage.out

coverage:
	go tool cover -html=coverage.out

build:
	go build

build-windows:
	GOOS=windows GOARCH=amd64 go build -o am-budget-view.exe

build-macos-amd64:
	GOOS=darwin GOARCH=amd64 go build -o am-budget-view-macos-amd64
