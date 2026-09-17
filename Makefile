.PHONY: test run seed docker-up docker-down tidy

tidy:
	go mod tidy

test:
	go test ./...

seed:
	go run ./cmd/seed

run:
	go run ./cmd/server

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
