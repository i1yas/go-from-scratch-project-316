build:
	go build -o bin/hexlet-go-crawler ./cmd/hexlet-go-crawler

run:
	go run ./cmd/hexlet-go-crawler $(URL)

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix
	
format:
	golangci-lint fmt

test:
	go test -race -coverpkg=./...  -coverprofile=coverage.out ./...

coverage: test
	go tool cover -func=coverage.out

coverage-html: test
	go tool cover -html=coverage.out
	
clean:
	rm -rf ./bin coverage.out

.PHONY: build lint lint-fix format test coverage coverage-html benchmark clean
