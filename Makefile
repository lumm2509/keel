lint:
	golangci-lint run -c ./golangci.yml ./...

test:
	go test -race ./... -v --cover

jstypes:
	go run ./plugins/jsvm/internal/types/types.go

test-report:
	go test -race ./... -v --cover -coverprofile=coverage.out
	go tool cover -html=coverage.out
