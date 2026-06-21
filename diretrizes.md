# Minhas diretrizes para o desafio de SRE da Foxbit

Este documento reúne as **diretrizes que eu (candidato) defini** para resolver o
desafio, **além** do que o enunciado exige (ver [`teste-vaga-sre-foxbit.md`](./teste-vaga-sre-foxbit.md)).
Elas guiaram a implementação do início ao fim.

> Este é o documento **final**: ao longo do trabalho algumas diretrizes foram
> ajustadas, incluídas ou removidas conforme as decisões de design amadureceram.
> A narrativa de *como* cheguei até aqui (e o uso do Claude Code) está em
> [`PARA-O-AVALIADOR.md`](./PARA-O-AVALIADOR.md).

## Princípios gerais

- **Simplicidade** — resolver o problema sem complexidade desnecessária; cada
  peça deve ser fácil de entender, operar e remover.
- **Boas práticas por artefato** — código, testes, Dockerfile, chart e pipelines
  seguem as boas práticas da sua respectiva comunidade.
- **Contract-first** — a API é definida antes da implementação.
- **Orientado a testes** — cobertura relevante no núcleo, testes rodando no CI.
- **Observabilidade** — logs estruturados (JSON) e métricas Prometheus.
- **Segurança** — superfície mínima (imagem distroless, não-root, FS read-only),
  varredura de segredos no CI, sem credenciais versionadas.
- **Documentação** — cada repositório se documenta e referencia os demais, sem
  duplicar conteúdo.

## 1. Aplicação web

- **Stack** — **Go + [Echo](https://echo.labstack.com/)**, com **Redis** para o
  cache. (A primeira versão destas diretrizes dizia "Go + Fiber"; troquei para
  Echo porque o `oapi-codegen` gera um *server interface* nativo para Echo, o que
  reforça o contract-first.)
- **API REST** seguindo boas práticas de design.
- **OpenAPI** como contrato (fonte da verdade) e **`oapi-codegen`** para gerar
  tipos e a interface do servidor a partir dele.
- **Cobertura de testes** com relatório publicado como artefato no CI.
- **Containerização** com **Docker** (imagem multi-stage, distroless, não-root).
- **Métricas** Prometheus expostas em `/metrics`.
- **Logging** em **JSON** (via `slog`), pronto para coleta centralizada. Os
  endpoints operacionais (`/healthz`, `/readyz`, `/metrics`) logam em `debug`
  para não poluir o log com o ruído das probes/scrape.
- **Cache** para evitar reprocessar operações repetidas:
  - Para tornar o cache observável, cada *cache miss* aplica um **delay simulado
    de 5s** (operação "custosa"); um *hit* responde instantaneamente.
  - Entradas **expiram por TTL** (ex.: 1 min).
  - Implementado com **Redis**.
  - Controlado por uma **flag** (variável de ambiente), **desabilitada por
    padrão**.
  - **Degradação graciosa**: se a flag estiver ligada mas faltar `REDIS_ADDR`, ou
    o Redis estiver inacessível, a aplicação **loga e continua sem cache** — nunca
    cai por causa do cache. Erros de Redis por operação contam como *miss*. A
    aplicação **reconecta sob demanda**: se o Redis ficar disponível depois (ex.:
    subiu junto com a app no Kubernetes), o cache passa a funcionar **sem precisar
    reiniciar**.
  - **Logs de cache**: status no boot (habilitado/desabilitado/erro) e eventos de
    *hit*/*miss*/*update* com os termos e o resultado.
  - **Métricas de cache**: hits, misses e sets, por operação e no geral.

### Desvio consciente do contrato (`result`)

O enunciado mostra `{ "result": <int> }`, mas divisão raramente é inteira. Para
não perder informação, a API aceita termos decimais e um parâmetro **opcional
`precision`** (casas decimais, **truncadas sem arredondar**): sem ele, o resultado
é um inteiro truncado (compatível com o exemplo do enunciado); com ele, um decimal
truncado. Divisão por zero retorna `400`.

## 2. Empacotamento e deploy em Kubernetes

- **Helm chart genérico** chamado **`microservice`** (e não acoplado ao nome da
  app), reutilizável para qualquer microsserviço HTTP stateless.
- **Boas práticas de Kubernetes**: ConfigMap/Secret, Deployment, Service.
- **Service `ClusterIP`** — a app é acessível **somente dentro do cluster**
  (requisito do enunciado).
- **Probes** startup, liveness (`/healthz`) e readiness (`/readyz`) — readiness
  **não depende do Redis**.
- **Requests/limits** de CPU e memória configurados.
- **Security context** não-root, FS read-only, capabilities removidas.
- **Monitoramento** com **ServiceMonitor opcional**: o chart cria o ServiceMonitor
  **apenas se** o CRD do Prometheus Operator (`monitoring.coreos.com/v1`) estiver
  disponível no cluster (checagem via `.Capabilities.APIVersions`); caso contrário
  a instalação **prossegue sem ele** e avisa nas `NOTES`. Vale para a app e para o
  Redis interno.
- **Cache no chart**:
  - Bloco de configuração com flag principal de habilitar/desabilitar.
  - **Redis interno** (via *subchart* oficial `bitnami/redis`, standalone, sem
    persistência, com exporter de métricas) **ou externo** (endereço + credenciais
    via Secret).
- **`values.yaml` agnóstico** a ambiente/app, com seções comentadas (imagem,
  cache, monitoramento, recursos, segurança, probes, logging).
- **Testes do chart** com `helm lint` e `helm unittest`.

## 3. Organização em múltiplos repositórios

*(Esta seção substitui a diretriz inicial de manter chart e values na raiz do
repositório da aplicação.)* Comecei com tudo num único repositório, mas os
pipelines ficaram acoplados (um push mexendo em app e chart disparava builds e
deploys cruzados). Separei em **3 repositórios**, cada um com responsabilidade e
CI/CD próprios:

| Repositório | Responsabilidade |
| --- | --- |
| [`foxbit-calc`](https://github.com/fernandosoaresjr/foxbit-calc) | a aplicação Go + imagem Docker. |
| [`foxbit-charts`](https://github.com/fernandosoaresjr/foxbit-charts) | o chart `microservice` (publicado como artefato **OCI** no GHCR). |
| [`foxbit-calc-k8s`](https://github.com/fernandosoaresjr/foxbit-calc-k8s) | os `values.yaml`/`version.yaml` de deploy (estado do que está implantado). |

## 4. CI/CD

- **GitHub Actions**, com **cada repositório tendo seus próprios workflows**
  (`CI`, `CD` e `Secret Scan`), acionados por eventos específicos (push na `main`,
  pull request) e por *paths*.
- **`foxbit-calc`** — `CI`: fmt, vet, lint do contrato (Spectral), conferência do
  código gerado, testes + cobertura, e **publicação da imagem no GHCR só na
  `main`**. `CD`: ao concluir o CI, **promove** a tag da imagem commitando em
  `foxbit-calc-k8s/k8s/version.yaml` (GitOps).
- **`foxbit-charts`** — `CI`: `helm lint` + `helm unittest`, com **gate de versão**
  (PR que altera o chart precisa bumpar `Chart.yaml`). `CD`: empacota e **publica o
  chart como OCI** no GHCR e cria uma **GitHub Release com changelog**.
- **`foxbit-calc-k8s`** — `CI`: baixa o chart publicado (OCI), renderiza com os
  values/version e valida o schema com **kubeconform**. `CD`: **`helm upgrade
  --install`** no cluster, com verificação de rollout e *smoke test* dentro do
  cluster.
- **Secret Scan** (gitleaks) em todo push/PR nos 3 repos.
- **Documentação dos workflows** em `.github/workflows/README.md`, referenciada
  nos READMEs.

## 5. Fluxo de desenvolvimento

- Iterativo e incremental: app → chart → CI/CD, validando cada parte.
- **Conventional Commits** para um histórico legível.
- Desenvolvimento orientado a testes, com cobertura relevante no núcleo
  (`calculator`, `config`, `observability` ~100%; `service` ~97%; `cache` ~83%).

## 6. Entrega

- O avaliador deve conseguir **fazer deploy, validar, alterar+redeployar e
  remover** a aplicação seguindo a documentação.
- O caminho recomendado é **fork dos 3 repositórios**, ajustar nomes/registry,
  configurar os secrets e deixar o **pipeline** fazer o deploy.
- Um guia passo a passo está em [`PARA-O-AVALIADOR.md`](./PARA-O-AVALIADOR.md).
