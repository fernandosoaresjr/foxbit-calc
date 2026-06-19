# Helm Chart — foxbit-calc

Chart para fazer deploy da API **Foxbit Calc** em um cluster Kubernetes, com
cache Redis opcional (interno ou externo) e integração opcional com o
Prometheus Operator.

## Índice

- [Introdução](#introdução)
- [Pré-requisitos](#pré-requisitos)
- [Instalação](#instalação)
- [Configuração (`values.yaml`)](#configuração-valuesyaml)
  - [Cache](#cache)
  - [Monitoramento](#monitoramento)
  - [Segurança](#segurança)
  - [Probes](#probes)
- [Acessando a aplicação](#acessando-a-aplicação)
- [Atualização e remoção](#atualização-e-remoção)
- [Testes do chart](#testes-do-chart)
- [Decisões de design](#decisões-de-design)

## Introdução

O chart cria os seguintes recursos:

| Recurso | Descrição |
| --- | --- |
| `Deployment` | A aplicação (não-root, root FS somente leitura). |
| `Service` (ClusterIP) | Expõe a app **somente dentro do cluster**, na porta 8000. |
| `ConfigMap` | Configuração não sensível (log, delay, cache). |
| `Secret` | Senha do Redis externo (apenas quando aplicável). |
| `ServiceMonitor` | Opcional — só quando o CRD do Prometheus Operator existe. |
| Subchart `bitnami/redis` | Opcional — Redis interno para cache. |

## Pré-requisitos

- Kubernetes 1.23+
- Helm 3.8+
- (Opcional) Prometheus Operator, para o `ServiceMonitor`.

## Instalação

O chart declara o subchart `bitnami/redis` como dependência. Baixe-a antes de
instalar:

```bash
helm dependency build ./chart
```

Instale com os valores padrão (cache desabilitado):

```bash
helm upgrade --install foxbit-calc ./chart \
  -n foxbit-calc --create-namespace
```

Ou usando o exemplo pronto (cache interno habilitado):

```bash
helm upgrade --install foxbit-calc ./chart \
  -n foxbit-calc --create-namespace \
  -f k8s/values.yaml
```

## Configuração (`values.yaml`)

Principais parâmetros (veja `chart/values.yaml` para a lista completa e
comentada):

| Parâmetro | Default | Descrição |
| --- | --- | --- |
| `image.repository` | `ghcr.io/fernandosoaresjr/foxbit-calc` | Imagem da aplicação. |
| `image.tag` | `""` | Tag; vazio usa `Chart.AppVersion`. |
| `replicaCount` | `2` | Réplicas da aplicação. |
| `service.type` | `ClusterIP` | Mantenha `ClusterIP` (acesso interno). |
| `service.port` | `8000` | Porta da aplicação. |
| `calcDelay` | `5s` | Delay simulado (operação custosa). |
| `logging.level` / `logging.format` | `info` / `json` | Logging. |
| `cache.enabled` | `false` | Liga/desliga o cache. |
| `cache.type` | `internal` | `internal` (subchart) ou `external`. |
| `cache.ttl` | `60s` | Expiração das entradas. |
| `serviceMonitor.enabled` | `false` | Cria `ServiceMonitor` (se houver CRD). |
| `resources` | requests/limits | CPU/memória. |

### Cache

**Interno** (subchart `bitnami/redis`) — habilite `cache` e o subchart:

```yaml
cache:
  enabled: true
  type: internal
  ttl: "60s"
redis:
  enabled: true        # obrigatório no modo internal
```

**Externo** (Redis gerenciado/existente):

```yaml
cache:
  enabled: true
  type: external
  external:
    address: "meu-redis:6379"
    existingSecret: "meu-redis-auth"   # chave: redis-password
```

> O chart **valida** a configuração: `type: internal` exige `redis.enabled: true`;
> `type: external` exige `external.address`. Configurações inconsistentes abortam
> a instalação com uma mensagem clara.

### Monitoramento

Com `serviceMonitor.enabled: true`, o chart cria um `ServiceMonitor` **apenas se**
o CRD `monitoring.coreos.com/v1` existir no cluster. Caso contrário, a instalação
**prossegue** sem o `ServiceMonitor` e exibe um aviso (ver `NOTES`). No modo de
cache interno, o `redis.metrics` expõe métricas do Redis para o Prometheus.

### Segurança

O pod roda como usuário não-root (uid 65532), com `readOnlyRootFilesystem`,
`allowPrivilegeEscalation: false`, todas as capabilities removidas e
`seccompProfile: RuntimeDefault`.

### Probes

`startupProbe`, `livenessProbe` (`/healthz`) e `readinessProbe` (`/readyz`)
configuráveis. **Os endpoints de saúde não dependem do Redis** — a aplicação
continua pronta mesmo sem cache.

## Acessando a aplicação

Como o `Service` é `ClusterIP`, acesse via `port-forward`:

```bash
kubectl port-forward -n foxbit-calc svc/foxbit-calc 8000:8000
curl "http://localhost:8000/api/sum?term_one=4&term_two=1"   # {"result":5}
```

## Atualização e remoção

```bash
# Atualizar (ex.: nova tag de imagem)
helm upgrade foxbit-calc ./chart -n foxbit-calc -f k8s/values.yaml \
  --set image.tag=<nova-tag>

# Remover
helm uninstall foxbit-calc -n foxbit-calc
```

## Testes do chart

Lint e testes unitários (via [helm-unittest](https://github.com/helm-unittest/helm-unittest)):

```bash
helm lint ./chart
helm unittest ./chart
```

## Decisões de design

- **ClusterIP, sem Ingress** — atende ao requisito de acesso somente interno.
- **Subchart bitnami/redis** — reaproveita um chart de Redis mantido, com
  exporter de métricas e `ServiceMonitor` próprios.
- **Validação de cache no template** — falha cedo, com mensagem clara, em vez de
  gerar um deploy quebrado.
- **`ServiceMonitor` condicionado ao CRD** — o chart é portável entre clusters
  com e sem Prometheus Operator.
- **Caveat Bitnami** — o catálogo de imagens da Bitnami mudou em 2025; a versão
  do subchart está fixada (`27.0.10`). Se o cluster não conseguir baixar a imagem
  do Redis, sobrescreva o registro (ex.: `redis.image.registry`) conforme a
  política do seu ambiente.
```
