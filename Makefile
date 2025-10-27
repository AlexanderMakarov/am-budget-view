run:
	go run .

run-help:
	go run . --help

test:
	go test -coverprofile coverage.out

coverage:
	go tool cover -html=coverage.out

test-coverage: test coverage

generate-demo:
	python3.12 scripts/generate_demo.py

generate-demo-plots:
	python3.12 scripts/generate_demo.py --plots

bank-downloader:
	python3 scripts/bank_downloader.py

build:
	go build

build-windows:
	GOOS=windows GOARCH=amd64 go build -o am-budget-view.exe

build-macos-amd64:
	GOOS=darwin GOARCH=amd64 go build -o am-budget-view-macos-amd64

release:
	@if [ -z "$(version)" ]; then \
		echo "Error: version parameter is required. Use 'make release version=X.X.X [comment=\"Your comment\"]'"; \
		exit 1; \
	fi
	git fetch --all
	@echo "Last 5 release tags:"
	@git tag -l "release*" | sort -rV | head -n 5
	@echo ""
	git checkout master
	git pull
	git tag -a "release$(version)" -m "$(comment)"
	@echo "Now run: git push origin release$(version)"
