run:
	go run main.go

test:
	go test -v ./... -short

integration_test:
	go test -v ./... -run Integration