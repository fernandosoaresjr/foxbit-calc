# Makefile da aplicação Foxbit Calc.
# Use `make help` para listar os alvos disponíveis.

IMAGE        ?= ghcr.io/fernandosoaresjr/foxbit-calc
TAG          ?= dev
OAPI_CODEGEN := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1

.PHONY: help generate test cover cover-html vet fmt lint build run docker-build tidy clean

help: ## Lista os alvos disponíveis
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

generate: ## Gera o código do servidor/types a partir do contrato OpenAPI
	$(OAPI_CODEGEN) -config oapi-codegen.yaml api/openapi.yaml

test: ## Roda todos os testes (com detector de race)
	go test ./... -race -count=1

cover: ## Roda os testes gerando relatório de cobertura
	go test ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -func=coverage.out | tail -1

cover-html: cover ## Gera o relatório de cobertura em HTML (coverage.html)
	go tool cover -html=coverage.out -o coverage.html

vet: ## Roda go vet
	go vet ./...

fmt: ## Formata o código
	gofmt -l -w .

lint: ## Roda o golangci-lint (requer a ferramenta instalada)
	golangci-lint run

build: ## Compila o binário em bin/server
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

run: ## Roda a aplicação localmente
	go run ./cmd/server

docker-build: ## Constrói a imagem Docker ($(IMAGE):$(TAG))
	docker build -t $(IMAGE):$(TAG) .

tidy: ## Atualiza go.mod/go.sum
	go mod tidy

clean: ## Remove artefatos de build e cobertura
	rm -rf bin coverage.out coverage.html
