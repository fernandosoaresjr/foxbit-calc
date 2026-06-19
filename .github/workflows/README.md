# Workflows de CI/CD

Quatro workflows do GitHub Actions automatizam validação, build, teste e deploy.

| Workflow | Arquivo | Dispara em | O que faz |
| --- | --- | --- | --- |
| **App CI** | `app-ci.yaml` | push/PR (código da app) | fmt, vet, lint OpenAPI (Spectral), codegen, testes+cobertura; publica a imagem no GHCR (só na `main`). |
| **Chart CI** | `chart-ci.yaml` | push/PR (`chart/`, `k8s/`) | `helm lint`, `helm unittest`, render de sanidade. |
| **Deploy** | `deploy.yaml` | push na `main` (app **ou** chart/values), ou manual | `helm upgrade --install` no cluster + verificação. |
| **Secret Scan** | `secret-scan.yaml` | todo push/PR | varre o repositório por credenciais commitadas (gitleaks). |

## Introdução

O **App CI** valida e publica a imagem; o **Chart CI** valida o chart; o
**Secret Scan** varre segredos. O **Deploy** dispara no `push` para a `main`
quando muda o app **ou** o `chart/`/`k8s/`. Como um merge é **um único push**,
há sempre **um único deploy**, mesmo que o PR tenha alterado app e chart juntos.

Ordem imagem→deploy: o Deploy roda em paralelo ao App CI. Se o **app** mudou, ele
usa a tag do commit (`sha`) e **aguarda a imagem** ser publicada no GHCR antes de
implantar. Se só o **chart/values** mudou, implanta a imagem `latest` existente
(a aplicação não mudou).

## Configuração

### Permissões

`App CI` usa o `GITHUB_TOKEN` (com `packages: write`) para publicar no GHCR —
nenhum segredo extra é necessário para a imagem.

### Secrets

| Secret | Usado por | Obrigatório | Descrição |
| --- | --- | --- | --- |
| `KUBE_CONFIG` | Deploy | sim | `kubeconfig` do cluster, **codificado em base64**. |
| `GHCR_PULL_TOKEN` | Deploy | não | PAT com `read:packages` para o cluster baixar a imagem privada do GHCR. Omita se a imagem for pública. |

Gere o `KUBE_CONFIG` a partir do seu `kubeconfig`:

```bash
base64 -w0 ~/.kube/config   # Linux
base64    ~/.kube/config    # macOS
```

Cole o resultado em **Settings → Secrets and variables → Actions → New
repository secret** com o nome `KUBE_CONFIG`.

> O cluster precisa ser **acessível pela internet** (a partir dos runners do
> GitHub). Se o cluster da Foxbit não for exposto, use o caminho de deploy
> **manual** (`helm install`) descrito no README principal — ele não depende
> deste workflow.

### Imagem privada no GHCR

Por padrão a imagem publicada no GHCR é privada e o cluster precisa de
credenciais para baixá-la. Há três opções:

1. **Pacote público** (mais simples) — torne o pacote público em **Packages →
   Package settings**. Nenhum secret é necessário.
2. **Pull secret gerenciado pelo chart** (recomendado para registry privado) —
   configure o secret `GHCR_PULL_TOKEN` (PAT com `read:packages`). O workflow de
   Deploy passa as credenciais ao Helm via `--set`, e o chart cria um Secret
   `kubernetes.io/dockerconfigjson` no cluster, referenciando-o automaticamente.
   As credenciais **nunca** ficam no `values.yaml`.
3. **Pull secret pré-existente** — crie a Secret manualmente
   (`kubectl create secret docker-registry ...`) e referencie-a em
   `imagePullSecrets` no `k8s/values.yaml`.

> Evite usar o `GITHUB_TOKEN` do job como credencial de pull do cluster: ele
> **expira ao fim do workflow**, e o nó falharia ao re-baixar a imagem depois.

## Uso

- **App CI / Chart CI / Secret Scan**: acionados automaticamente por push e pull
  request nos caminhos relevantes. Veja os logs em **Actions**; a cobertura fica
  em *Artifacts* (`coverage`).
- **Deploy**: roda no push para a `main` quando muda o app ou o chart/values.
  Para implantar uma tag específica manualmente, use **Run workflow** (input
  `image_tag`).

## Arquitetura

```
                    ┌─> App CI (test sempre; imagem no GHCR só na main)
push/PR ───────────>├─> Chart CI (helm lint + unittest)
                    └─> Secret Scan (gitleaks)

push main (app OU chart/k8s) ──> Deploy ──> [app mudou? aguarda imagem :sha
                                             senão usa :latest] ──> helm upgrade
                                             ──> rollout + smoke test no cluster
```

- **App CI** separa `test` (sempre) de `image` (só `main`), evitando publicar
  imagens de PR.
- **Deploy por `push` (não `workflow_run`)** — um merge é um único push, logo um
  único deploy, mesmo quando o PR altera app e chart juntos. A detecção de
  mudança (`paths-filter`) decide a tag e se deve aguardar a imagem.
- **Verificação** roda `rollout status` e um *smoke test* dentro do cluster
  (pod efêmero `curl` no Service `ClusterIP`), com coleta de diagnósticos em
  caso de falha.

## Decisões de design

- **GitHub Actions** — nativo ao repositório, sem infraestrutura adicional.
- **Deploy disparado por `push` com `paths` de app+chart+k8s** — garante deploy
  em mudanças de app **ou** chart, e exatamente **um** deploy por merge (sem o
  duplo disparo que `workflow_run` de dois workflows causaria). `concurrency`
  enfileira deploys para nunca rodarem sobrepostos.
- **Espera ativa da imagem** — desacopla build (App CI) de deploy sem corrida:
  o Deploy só implanta `:sha` depois que a imagem aparece no GHCR.
- **Publicação só na `main`** — PRs validam, mas não publicam imagens nem
  implantam, protegendo o ambiente.
- **Lint de contrato (Spectral) + secret scan (gitleaks)** — qualidade de API e
  segurança verificadas em todo PR, antes do merge.
- **GHCR** — integrado ao GitHub e ao `GITHUB_TOKEN`, sem credenciais externas.
