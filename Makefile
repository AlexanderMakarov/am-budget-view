run:
	go run .

# Need `go install github.com/air-verse/air@latest`.
# For settings see https://github.com/air-verse/air/blob/master/air_example.toml
live:
	air --build.cmd "go build -o ./__debug_bin ." --build.bin "./__debug_bin" \
		--build.delay 2000 --build.exclude_dir "testdata"

test:
	go test -coverprofile coverage.out

coverage:
	go tool cover -html=coverage.out

build:
	go build
