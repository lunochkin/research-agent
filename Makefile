# Load .env (optional) so targets like `migrate` see the same vars as the binary.
-include .env
export

DATABASE_URL ?= postgres://rag:rag@localhost:5432/rag?sslmode=disable

.PHONY: db migrate build ingest ask test fmt tidy

db: ## start postgres+pgvector
	docker compose up -d

migrate: ## apply schema
	psql "$(DATABASE_URL)" -f migrations/001_init.sql

build:
	go build -o bin/research-agent ./cmd/research-agent

ingest: build ## ingest a kaggle arxiv dump: make ingest FILE=path.json
	./bin/research-agent ingest --file "$(FILE)"

ask: build ## ask a question: make ask Q="your question"
	./bin/research-agent ask "$(Q)"

test:
	go test ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy
