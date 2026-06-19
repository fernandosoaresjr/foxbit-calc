# Workflows de CI/CD

Três workflows do GitHub Actions automatizam build, teste e deploy.

| Workflow | Arquivo | Dispara em | O que faz |
| --- | --- | --- | --- |
| **App CI** | `app-ci.yaml` | push/PR (código da app) | fmt, vet, codegen, testes+cobertura; publica a imagem no GHCR (só na `main`). |
| **Chart CI** | `chart-ci.yaml` | push/PR (`chart/`, `k8s/`) | `helm lint`, `helm unittest`, render de sanidade. |
| **Deploy** | `deploy.yaml` | após "App CI" na `main`, ou manual | `helm upgrade --install` no cluster + verificação. |

## Introdução

O fluxo segue a ordem: **App CI** valida e publica a imagem → **Deploy** (via
`workflow_run`) implanta a imagem recém-publicada no cluster. **Chart CI** é
independente e valida o chart em qualquer mudança.

## Configuração

### Permissões

`App CI` usa o `GITHUB_TOKEN` (com `packages: write`) para publicar no GHCR —
nenhum segredo extra é necessário para a imagem.

### Secrets

| Secret | Usado por | Descrição |
| --- | --- | --- |
| `KUBE_CONFIG` | Deploy | `kubeconfig` do cluster, **codificado em base64**. |

Gere o secret a partir do seu `kubeconfig`:

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

Por padrão a imagem publicada no GHCR é privada. Para o cluster baixá-la, crie um
`imagePullSecret` e referencie-o em `imagePullSecrets` no `k8s/values.yaml`, ou
torne o pacote público em **Packages → Package settings**.

## Uso

- **App CI / Chart CI**: acionados automaticamente por push e pull request nos
  caminhos relevantes. Veja os logs em **Actions**; a cobertura fica em
  *Artifacts* (`coverage`).
- **Deploy**: roda sozinho após o App CI na `main`. Para implantar uma tag
  específica manualmente, use **Run workflow** (input `image_tag`).

## Arquitetura

```
push main ──> App CI ──(imagem no GHCR)──> workflow_run ──> Deploy ──> cluster
   │                                                          │
   └────────────> Chart CI (lint + unittest)                 └─> verificação (rollout + smoke test)
```

- **App CI** separa `test` (sempre) de `image` (só `main`), evitando publicar
  imagens de PR.
- **Deploy** resolve a tag a partir do `head_sha` do App CI, garantindo que a
  imagem implantada é exatamente a que passou no CI.
- **Verificação** roda `rollout status` e um *smoke test* dentro do cluster
  (pod efêmero `curl` no Service `ClusterIP`), com coleta de diagnósticos em
  caso de falha.

## Decisões de design

- **GitHub Actions** — nativo ao repositório, sem infraestrutura adicional.
- **`workflow_run` em vez de um job único** — desacopla CI de CD e garante a
  ordem imagem→deploy sem corrida.
- **Publicação só na `main`** — PRs validam, mas não publicam imagens nem
  implantam, protegendo o ambiente.
- **GHCR** — integrado ao GitHub e ao `GITHUB_TOKEN`, sem credenciais externas.
