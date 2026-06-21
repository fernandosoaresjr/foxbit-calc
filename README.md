# Foxbit Calc

API REST das quatro operações básicas da matemática (adição, subtração,
multiplicação e divisão), em **Go + Echo**, com cache **Redis** opcional,
observabilidade (logs JSON + métricas Prometheus) e **CI/CD** com GitHub Actions.

Solução do [desafio técnico de SRE](./teste-vaga-sre-foxbit.md). O deploy em
Kubernetes vive em repositórios separados — ver [Repositórios relacionados](#repositórios-relacionados).

> 📋 **Avaliador(a):** comece por **[`PARA-O-AVALIADOR.md`](./PARA-O-AVALIADOR.md)** —
> visão geral, decisões de arquitetura e o roteiro de teste E2E. As diretrizes que
> guiaram a solução estão em [`diretrizes.md`](./diretrizes.md).

## Índice

- [Foxbit Calc](#foxbit-calc)
  - [Índice](#índice)
  - [Arquitetura](#arquitetura)
  - [Repositórios relacionados](#repositórios-relacionados)
  - [API](#api)
  - [Desenvolvimento local](#desenvolvimento-local)
  - [Configuração (variáveis de ambiente)](#configuração-variáveis-de-ambiente)
  - [Docker](#docker)
  - [Deploy](#deploy)
  - [Observabilidade](#observabilidade)
  - [Testes e cobertura](#testes-e-cobertura)
  - [Decisões de design](#decisões-de-design)

## Arquitetura

```
cmd/server          → bootstrap (config, logger, métricas, cache, HTTP)
internal/
  api               → contrato gerado (oapi-codegen) + handlers + router
  service           → orquestra delay + cache + métricas (cola)
  calculator        → operações puras + truncamento (núcleo)
  cache             → interface Cache + RedisCache + NoopCache
  config            → carga de configuração via env
  observability     → logger JSON (slog) + métricas Prometheus
api/openapi.yaml    → contrato REST (fonte da verdade, contract-first)
.github/workflows/  → CI/CD (ver .github/workflows/README.md)
```

Fluxo de uma requisição:
`Echo (gerado) → handler → service → calculator`, com o `service` consultando o
cache e aplicando o delay (operação custosa simulada) apenas em *cache miss*.

## Repositórios relacionados

Este repositório contém **apenas a aplicação**. O deploy em Kubernetes foi separado em repositórios próprios, num fluxo GitOps:

| Repositório                                                              | Papel                                                              |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------ |
| [`foxbit-calc`](https://github.com/fernandosoaresjr/foxbit-calc)         | **(este)** a aplicação Go + imagem Docker.                         |
| [`foxbit-charts`](https://github.com/fernandosoaresjr/foxbit-charts)     | o Helm chart `microservice` (publicado como artefato OCI no GHCR). |
| [`foxbit-calc-k8s`](https://github.com/fernandosoaresjr/foxbit-calc-k8s) | os `values.yaml`/`version.yaml` de deploy que consomem o chart.    |

## API

A aplicação roda na porta **8000**. Endpoints de cálculo:

| Método | Rota                           | Operação      |
| ------ | ------------------------------ | ------------- |
| GET    | `/api/sum?term_one=&term_two=` | adição        |
| GET    | `/api/sub?term_one=&term_two=` | subtração     |
| GET    | `/api/mul?term_one=&term_two=` | multiplicação |
| GET    | `/api/div?term_one=&term_two=` | divisão       |

Parâmetros:

- `term_one`, `term_two` (obrigatórios): números (aceitam decimais).
- `precision` (opcional): casas decimais do resultado, **truncado sem
  arredondar**. Ausente ⇒ resultado **inteiro** (truncado).

Resposta: `{ "result": <number> }`. Exemplos:

```bash
GET /api/sub?term_one=4&term_two=1            -> {"result":3}
GET /api/div?term_one=4&term_two=3            -> {"result":1}
GET /api/div?term_one=4&term_two=3&precision=2 -> {"result":1.33}
GET /api/div?term_one=1&term_two=0            -> 400 {"message":"term_two must not be zero"}
```

Endpoints operacionais (fora do contrato, sem o delay):

| Rota       | Descrição                                                   |
| ---------- | ----------------------------------------------------------- |
| `/healthz` | Liveness — processo vivo.                                   |
| `/readyz`  | Readiness — pronto para tráfego (**não depende do Redis**). |
| `/metrics` | Métricas Prometheus.                                        |

O contrato completo está em [`api/openapi.yaml`](./api/openapi.yaml).

## Desenvolvimento local

Pré-requisitos: **Go 1.22+** (e, opcionalmente, Docker).

```bash
make run          # sobe em http://localhost:8000 (delay 5s, cache off)
# em outro terminal:
curl "http://localhost:8000/api/sum?term_one=4&term_two=1"
```

Para iterar sem o delay: `CALC_DELAY=0s make run`.

Demais alvos: `make help` (test, cover, cover-html, generate, build, docker-build, lint).

Regenerar o código a partir do contrato (após editar `api/openapi.yaml`):

```bash
make generate     # atualiza internal/api/api.gen.go (handlers escritos à mão são preservados)
```

## Configuração (variáveis de ambiente)

| Variável         | Default | Descrição                                   |
| ---------------- | ------- | ------------------------------------------- |
| `PORT`           | `8000`  | Porta HTTP.                                 |
| `LOG_LEVEL`      | `info`  | `debug`/`info`/`warn`/`error`.              |
| `LOG_FORMAT`     | `json`  | `json` ou `text`.                           |
| `CALC_DELAY`     | `5s`    | Delay simulado em cache miss.               |
| `CACHE_ENABLED`  | `false` | Liga/desliga o cache.                       |
| `CACHE_TTL`      | `60s`   | Expiração das entradas de cache.            |
| `REDIS_ADDR`     | —       | `host:port` do Redis (se cache habilitado). |
| `REDIS_PASSWORD` | —       | Senha do Redis (opcional).                  |
| `REDIS_DB`       | `0`     | Índice do banco Redis.                      |

Comportamento do cache na inicialização (degradação graciosa): se habilitado mas
`REDIS_ADDR` faltar ou o Redis estiver inacessível, a aplicação **loga um erro e
continua sem cache**. O status do cache é logado no boot.

Testar o cache localmente (com Docker):

```bash
docker run -d --rm --name redis -p 6379:6379 redis:7-alpine
CALC_DELAY=3s CACHE_ENABLED=true REDIS_ADDR=localhost:6379 make run
# 1ª chamada ~3s (miss); a 2ª (mesmos termos) é instantânea (hit).
curl "http://localhost:8000/api/mul?term_one=7&term_two=6"
```

## Docker

```bash
make docker-build                 # ghcr.io/fernandosoaresjr/foxbit-calc:dev
docker run --rm -p 8000:8000 -e CALC_DELAY=0s ghcr.io/fernandosoaresjr/foxbit-calc:dev
```

Imagem multi-stage baseada em `distroless/static:nonroot` (sem shell, usuário
não-root).

## Deploy

A aplicação é exposta por um `Service` **ClusterIP** — acessível **somente
dentro do cluster** (requisito do desafio). O chart e os values de deploy vivem
em repositórios separados (ver [Repositórios relacionados](#repositórios-relacionados)),
num fluxo **GitOps**:

```
push na main (este repo)
  └─ CI: publica ghcr.io/fernandosoaresjr/foxbit-calc:<sha>
       └─ CD: promove a tag para foxbit-calc-k8s/k8s/version.yaml
            └─ foxbit-calc-k8s: helm upgrade --install no cluster (chart OCI)
```

- **Configuração do CD deste repo**: secret `K8S_REPO_TOKEN` (fine-grained PAT
  com `contents: write` no repo `foxbit-calc-k8s`). Ver
  [`.github/workflows/README.md`](./.github/workflows/README.md).
- **Deploy/cluster** (chart, values, `KUBE_CONFIG`): ver o README de
  [`foxbit-calc-k8s`](https://github.com/fernandosoaresjr/foxbit-calc-k8s).

**Deploy manual** (qualquer cluster, sem CI/CD):

```bash
helm upgrade --install foxbit-calc \
  oci://ghcr.io/fernandosoaresjr/foxbit-charts/microservice --version 0.1.0 \
  -n foxbit-calc --create-namespace \
  --set image.repository=ghcr.io/fernandosoaresjr/foxbit-calc \
  --set image.tag=<tag> --set cache.enabled=true --set redis.enabled=true --wait

kubectl -n foxbit-calc port-forward svc/foxbit-calc-microservice 8000:8000
curl "http://localhost:8000/api/sum?term_one=4&term_two=1"   # {"result":5}
```

## Observabilidade

- **Logs** estruturados em JSON (`slog`): inclui requisições HTTP e eventos de
  cache (hit/miss/update) com termos e resultado.
- **Métricas** Prometheus em `/metrics`:
  - `calc_operations_total{operation}`
  - `calc_cache_hits_total{operation}`, `calc_cache_misses_total{operation}`,
    `calc_cache_sets_total{operation}`, `calc_cache_errors_total`
  - `calc_http_requests_total{method,route,status}`,
    `calc_http_request_duration_seconds{method,route}`
  - métricas padrão de Go/processo.

No Kubernetes, habilite `serviceMonitor.enabled` (se houver Prometheus Operator)
para coletar essas métricas, e o `redis.metrics` (cache interno) expõe métricas
do Redis.

## Testes e cobertura

```bash
make test         # go test ./... -race
make cover        # gera coverage.out e imprime o total
make cover-html   # gera coverage.html
```

Cobertura focada no núcleo: `calculator`, `config` e `observability` 100%;
`service` ~97%; `cache` ~83%.

## Decisões de design

- **Contract-first (OpenAPI + oapi-codegen)** — o contrato é a fonte da verdade;
  o servidor Echo e os tipos são gerados, e o compilador garante que os handlers
  acompanhem mudanças no contrato.
- **Parâmetro `precision` e tipo do `result`** — o desafio especifica
  `{"result": <int>}`, mas a divisão raramente é inteira. A solução aceita termos
  decimais e um `precision` opcional: sem ele, o resultado é um inteiro truncado
  (compatível com o exemplo do desafio); com ele, um decimal truncado. Divisão
  por zero retorna `400`.
- **Cache com degradação graciosa** — o cache nunca derruba a aplicação: erros de
  Redis viram *miss* e são contabilizados; readiness não depende do cache.
- **Delay simulado** — torna o valor do cache observável (5s no miss, instantâneo
  no hit), conforme as diretrizes do projeto.
- **Segurança** — imagem distroless não-root, root FS somente leitura,
  capabilities removidas; validação de entrada na borda.
- **Separação em multi-repo** — app, chart e values de deploy ficam em
  repositórios próprios (GitOps). Cada um tem seu CI/CD independente, evitando o
  acoplamento de pipelines de um monorepo. Ver
  [Repositórios relacionados](#repositórios-relacionados).
