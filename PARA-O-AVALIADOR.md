# Para o(s) avaliador(es) — desafio de SRE da Foxbit

Este documento complementa os READMEs de cada repositório. Ele explica **como a
solução foi construída**, **as decisões de arquitetura/organização** e dá um
**roteiro para testar tudo de ponta a ponta**.

- Enunciado do desafio: [`teste-vaga-sre-foxbit.md`](./teste-vaga-sre-foxbit.md)
- Minhas diretrizes (além do enunciado): [`diretrizes.md`](./diretrizes.md)

## Sumário

- [Para o(s) avaliador(es) — desafio de SRE da Foxbit](#para-os-avaliadores--desafio-de-sre-da-foxbit)
  - [Sumário](#sumário)
  - [Visão geral da solução](#visão-geral-da-solução)
  - [1. Como usei o Claude Code](#1-como-usei-o-claude-code)
  - [2. Decisões de arquitetura e organização](#2-decisões-de-arquitetura-e-organização)
  - [3. Como testar de ponta a ponta (E2E)](#3-como-testar-de-ponta-a-ponta-e2e)
    - [Caminho A — deploy direto no seu cluster (rápido)](#caminho-a--deploy-direto-no-seu-cluster-rápido)
    - [Caminho B — GitOps completo via fork + pipeline](#caminho-b--gitops-completo-via-fork--pipeline)
    - [Alterar a API e reimplantar](#alterar-a-api-e-reimplantar)
    - [Remover a aplicação](#remover-a-aplicação)

## Visão geral da solução

Uma API REST das 4 operações básicas (Go + Echo), com cache Redis opcional,
observabilidade (logs JSON + métricas Prometheus), empacotada num Helm chart
genérico e implantada em Kubernetes por um pipeline GitOps. A solução está
dividida em **3 repositórios**:

| Repositório | Papel | Artefato publicado |
| --- | --- | --- |
| [`foxbit-calc`](https://github.com/fernandosoaresjr/foxbit-calc) | aplicação Go | imagem `ghcr.io/fernandosoaresjr/foxbit-calc` |
| [`foxbit-charts`](https://github.com/fernandosoaresjr/foxbit-charts) | chart `microservice` | chart OCI `ghcr.io/fernandosoaresjr/foxbit-charts/microservice` |
| [`foxbit-calc-k8s`](https://github.com/fernandosoaresjr/foxbit-calc-k8s) | values/version de deploy | — (estado do deploy) |

> Validei o fluxo completo num cluster **GKE** real: push de código → CI publica a
> imagem → CD promove a tag para o repo de deploy → o deploy implanta no cluster e
> roda o smoke test. Os endpoints e o cache foram conferidos no cluster (miss ~5s,
> hit ~0,01s).

## 1. Como usei o Claude Code

Usei o **Claude Code** (agente de engenharia em terminal) como par de
desenvolvimento ao longo de todo o desafio. O fluxo foi:

1. **Insumos de contexto.** Dei ao agente dois documentos como base:
   - o enunciado [`teste-vaga-sre-foxbit.md`](./teste-vaga-sre-foxbit.md) (o *quê*);
   - as minhas [`diretrizes.md`](./diretrizes.md) (o *como* — stack, padrões,
     cache, segurança, CI/CD, organização).
   O agente tratou as diretrizes como a fonte da verdade das decisões e foi
   conferindo cada uma contra a implementação.

2. **Construção iterativa**, na ordem das diretrizes: primeiro a aplicação
   (contract-first com OpenAPI + `oapi-codegen`, núcleo de cálculo, cache com
   degradação graciosa, observabilidade), depois o Helm chart (com `helm
   unittest`), e por fim os pipelines de CI/CD. Commits seguindo *Conventional
   Commits*.

3. **Decisões revisadas em conjunto.** Algumas diretrizes evoluíram na conversa —
   por exemplo, a troca de **Fiber → Echo** (melhor integração com `oapi-codegen`)
   e, principalmente, a **quebra do monorepo em 3 repositórios** quando ficou
   claro que os pipelines acoplados eram um problema. O agente me apresentou os
   trade-offs (ex.: OCI vs GitHub Pages para o chart; PAT vs deploy key para o
   acesso cross-repo) e eu decidi.

4. **Verificação real, não só geração de código.** O agente não só escreveu
   código — ele rodou testes, `helm lint`/`template`, validou os workflows
   (`actionlint`) e, no fim, **provisionou um cluster GKE descartável e validou o
   deploy de ponta a ponta pelo próprio pipeline**, inclusive depurando problemas
   que só apareceram com o CI rodando de verdade:
   - `cmd/server/` estava sendo ignorado pelo `.gitignore` (padrão `server` amplo
     demais) — o entrypoint nunca tinha sido versionado;
   - faltava `setup-buildx-action` para o cache de build da imagem;
   - uma **corrida de inicialização** fazia a app subir antes do Redis e ficar
     presa sem cache — corrigido para reconectar sob demanda.

5. **Limites respeitados.** Quando uma ação exigia credencial inadequada (ex.:
   usar uma conta de nuvem corporativa para um recurso pessoal, ou um token amplo
   onde cabia um escopo mínimo), o agente parou e pediu confirmação em vez de
   prosseguir.

Em resumo: o Claude Code acelerou a implementação e a validação, mas as
**decisões de arquitetura foram conduzidas por mim**, com o enunciado e as
diretrizes como contrato.

## 2. Decisões de arquitetura e organização

**Stack — Go + Echo.** Go pela aderência à stack da Foxbit e binário enxuto;
Echo porque o `oapi-codegen` gera um *server interface* nativo para Echo,
reforçando o **contract-first** (o compilador garante que os handlers acompanhem
o contrato OpenAPI).

**Núcleo desacoplado.** `calculator` (operações puras) → `service` (orquestra
delay + cache + métricas) → `api` (HTTP). Isso mantém o núcleo testável (cobertura
~100% no núcleo) e o cache/observabilidade como detalhes de borda.

**Cache resiliente.** Cache Redis opcional com **degradação graciosa**: nunca
derruba a app; erros viram *miss*; e **reconecta sob demanda** (não fica preso em
"sem cache" se o Redis subir depois). O delay simulado de 5s torna o efeito do
cache observável.

**Desvio consciente do `result`.** O enunciado mostra inteiro; adicionei um
parâmetro opcional `precision` (truncado) para não perder a parte fracionária da
divisão, mantendo compatibilidade com o exemplo quando ausente.

**Três repositórios (em vez de monorepo).** Foi a principal decisão
organizacional. Um monorepo deixava os pipelines acoplados (mudança em app e chart
disparando builds/deploys cruzados, duplo disparo de deploy). Separando em
`foxbit-calc` (app), `foxbit-charts` (chart) e `foxbit-calc-k8s` (deploy), cada um
tem ciclo de vida e CI/CD independentes. É também como ambientes reais costumam
separar **código da aplicação**, **catálogo de charts** e **configuração de
deploy (GitOps)**.

**Chart genérico publicado via OCI.** O chart se chama `microservice` (não
acoplado à app) e é publicado como **artefato OCI no GHCR** — sem branch
`gh-pages`, mesmo registry da imagem, e com o subchart `bitnami/redis` já embutido
no pacote. O CI exige **bump de versão** no `Chart.yaml` em PRs que alteram o
chart, e o CD cria uma **release com changelog**.

**ServiceMonitor condicionado ao CRD.** O chart só cria o ServiceMonitor se o CRD
`monitoring.coreos.com/v1` existir no cluster (checagem via `.Capabilities`),
tornando o chart portável entre clusters com e sem Prometheus Operator.

**GitOps por promoção de tag.** O CD do `foxbit-calc` não faz deploy direto: ele
**commita a nova tag de imagem** em `foxbit-calc-k8s/k8s/version.yaml`. Esse repo
é a **fonte da verdade do que está implantado**; o push lá dispara o deploy. Assim
o histórico de deploys fica versionado e auditável.

**Segurança.** Imagem distroless não-root, FS read-only, capabilities removidas;
`Service` ClusterIP (acesso só interno, requisito do enunciado); varredura de
segredos (gitleaks) em todo push; nenhuma credencial versionada (tudo via
secrets do GitHub / Secrets do Kubernetes).

**Qualidade no CI.** `gofmt`/`go vet`, lint do contrato com **Spectral**, testes
com `-race` + cobertura, `helm lint` + `helm unittest`, e **kubeconform** validando
o schema dos manifests renderizados antes de qualquer deploy.

Detalhes de cada peça estão nos READMEs e em `.github/workflows/README.md` de cada
repositório.

## 3. Como testar de ponta a ponta (E2E)

Há dois caminhos. O **A** é o mais rápido para "fazer deploy, validar e remover"
no cluster de vocês. O **B** exercita todo o GitOps (CI → publish → promote →
deploy) com fork dos repositórios.

> Pré-requisitos comuns: `kubectl` e `helm` 3.8+ (suporte a OCI), e acesso a um
> cluster Kubernetes. A imagem e o chart estão **públicos** no GHCR, então o
> cluster os baixa sem credenciais.

### Caminho A — deploy direto no seu cluster (rápido)

Não exige fork nem secrets. Implanta o chart publicado, apontando para a imagem
publicada:

```bash
helm upgrade --install foxbit-calc \
  oci://ghcr.io/fernandosoaresjr/foxbit-charts/microservice --version 0.1.0 \
  -n foxbit-calc --create-namespace \
  --set image.repository=ghcr.io/fernandosoaresjr/foxbit-calc \
  --set image.tag=latest \
  --set cache.enabled=true --set redis.enabled=true \
  --wait

# rollout
kubectl -n foxbit-calc rollout status deploy/foxbit-calc-microservice

# validar (Service é ClusterIP → port-forward)
kubectl -n foxbit-calc port-forward svc/foxbit-calc-microservice 8000:8000 &
curl "http://localhost:8000/api/sum?term_one=4&term_two=1"        # {"result":5}
curl "http://localhost:8000/api/div?term_one=10&term_two=3&precision=2"  # {"result":3.33}
curl "http://localhost:8000/healthz"; curl "http://localhost:8000/metrics" | grep ^calc_

# cache em ação (1ª ~5s, 2ª instantânea)
time curl -s "http://localhost:8000/api/mul?term_one=7&term_two=6" >/dev/null
time curl -s "http://localhost:8000/api/mul?term_one=7&term_two=6" >/dev/null
```

### Caminho B — GitOps completo via fork + pipeline

Exercita o pipeline inteiro. Faça **fork dos 3 repositórios** para a sua conta/org
e ajuste as referências de `OWNER` (e do registry, se não usar o GHCR).

**B.1 — Ajustes de nome (troque `fernandosoaresjr` pelo seu owner):**

- `foxbit-calc-k8s/k8s/values.yaml` → `image.repository: <REGISTRY>/<OWNER>/foxbit-calc`
- `foxbit-calc-k8s/.github/workflows/{ci,cd}.yaml` → variável `CHART` aponta para
  `oci://<REGISTRY>/<OWNER>/foxbit-charts/microservice`
- `foxbit-calc/.github/workflows/cd.yaml` → `repository: <OWNER>/foxbit-calc-k8s`
  (repo de deploy a ser atualizado)
- Se trocar o registry (ex.: ECR/Docker Hub em vez do GHCR): ajuste o login e o
  caminho da imagem/chart nos workflows (`docker/login-action`, `helm registry
  login`) e torne os pacotes acessíveis pelo cluster (públicos ou via pull secret).

**B.2 — Secrets (em Settings → Secrets and variables → Actions):**

| Secret | Repositório | Para quê |
| --- | --- | --- |
| `K8S_REPO_TOKEN` | `foxbit-calc` | **fine-grained PAT** com *Contents: Read and write* **apenas** no fork de `foxbit-calc-k8s`. O CD usa para promover a tag de imagem. |
| `KUBE_CONFIG` | `foxbit-calc-k8s` | `kubeconfig` do cluster em **base64** (`base64 -w0 ~/.kube/config`). Use um endpoint **alcançável pelos runners do GitHub** (cluster gerenciado/público; um cluster local não é acessível pela nuvem do GitHub sem self-hosted runner). |

> Dica: prefira um kubeconfig com uma **ServiceAccount** dedicada (token) em vez de
> credenciais de admin pessoais.

**B.3 — Bootstrap (ordem importa):**

1. Faça um push em `foxbit-charts` (ex.: bump de `charts/microservice/Chart.yaml`
   `version`) → o **CD publica o chart OCI** e cria a release.
2. Em `foxbit-calc-k8s`, ajuste `k8s/version.yaml` (`chart.version` = a versão
   publicada) e os values → o **CI valida** (render + kubeconform).
3. Faça um push em `foxbit-calc` (qualquer mudança de código) → **CI publica a
   imagem** → **CD promove** a tag para `foxbit-calc-k8s` → **CD do k8s implanta**
   no cluster e roda o smoke test.

Acompanhe em **Actions** de cada repo; o deploy loga rollout e o smoke test
(`/healthz` + uma operação) de dentro do cluster.

### Alterar a API e reimplantar

(Critério de avaliação.) Edite o contrato/código em `foxbit-calc`, rode
`make generate && make test`, faça push para uma nova branch e abra um PR.
Após a aprovação e merge do PR na `main`, o pipeline cuida do resto
(imagem nova → promoção → deploy). Manualmente, equivale a publicar uma imagem
nova e `helm upgrade ... --set image.tag=<nova-tag>`.

### Remover a aplicação

```bash
helm uninstall foxbit-calc -n foxbit-calc
kubectl delete namespace foxbit-calc    # opcional
```
