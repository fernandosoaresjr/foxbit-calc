# Workflows de CI/CD — foxbit-calc

Três workflows do GitHub Actions cuidam da aplicação. O deploy em si vive em
outros repositórios (ver [Arquitetura multi-repo](#arquitetura-multi-repo)).

| Workflow | Arquivo | Dispara em | O que faz |
| --- | --- | --- | --- |
| **CI** | `ci.yaml` | push/PR (código da app) | fmt, vet, lint OpenAPI (Spectral), codegen, testes+cobertura; publica a imagem no GHCR (só na `main`). |
| **CD** | `cd.yaml` | `workflow_run` após o CI (na `main`) | promove a tag da imagem para o repo de deploy `foxbit-calc-k8s`. |
| **Secret Scan** | `secret-scan.yaml` | todo push/PR | varre o repositório por credenciais commitadas (gitleaks). |

## Arquitetura multi-repo

Este repositório contém **apenas a aplicação**. O chart e os values de deploy
foram separados, num fluxo GitOps:

```
foxbit-calc (este repo)
  └─ CI: testa + publica ghcr.io/fernandosoaresjr/foxbit-calc:<sha> (na main)
       └─ CD (workflow_run, após CI ok): commita image.tag=<sha> em
          foxbit-calc-k8s/k8s/version.yaml (push direto na main de lá)
            └─ foxbit-calc-k8s CD: helm upgrade --install no cluster
                 usando o chart `microservice` (foxbit-charts, via OCI)
```

- [`foxbit-charts`](https://github.com/fernandosoaresjr/foxbit-charts) — o chart
  `microservice` (publicado como artefato OCI no GHCR).
- [`foxbit-calc-k8s`](https://github.com/fernandosoaresjr/foxbit-calc-k8s) — os
  `values.yaml`/`version.yaml` de deploy.

## Configuração

### Permissões

`CI` usa o `GITHUB_TOKEN` (com `packages: write`) para publicar no GHCR — nenhum
segredo extra é necessário para a imagem.

### Secrets

| Secret | Usado por | Obrigatório | Descrição |
| --- | --- | --- | --- |
| `K8S_REPO_TOKEN` | CD | sim | **Fine-grained PAT** com `contents: write` **apenas** no repo `foxbit-calc-k8s`. O CD usa esse token para commitar a nova tag de imagem em `k8s/version.yaml`. Um push com PAT dispara o CD do repo de deploy (o `GITHUB_TOKEN` padrão não tem acesso a outro repo nem dispara workflows lá). |

Configure em **Settings → Secrets and variables → Actions → New repository
secret**. Para criar o PAT: **Settings → Developer settings → Fine-grained
tokens**, com acesso *Only select repositories → foxbit-calc-k8s* e permissão
*Repository permissions → Contents: Read and write*.

## Uso

- **CI / Secret Scan**: acionados por push e pull request nos caminhos
  relevantes. Logs em **Actions**; a cobertura fica em *Artifacts* (`coverage`).
- **CD**: dispara automaticamente quando o **CI** conclui com sucesso num push na
  `main`. Promove a imagem recém-publicada para o repo de deploy.

## Decisões de design

- **GitHub Actions** — nativo ao repositório, sem infraestrutura adicional.
- **CD via `workflow_run`** — garante que a imagem `:sha` já foi publicada pelo
  CI antes de promover a tag (sem corrida e sem espera ativa).
- **GitOps por commit no repo de deploy** — a tag promovida vira um commit em
  `foxbit-calc-k8s`, que é a fonte da verdade do que está implantado; o deploy é
  responsabilidade daquele repo.
- **Publicação só na `main`** — PRs validam, mas não publicam imagens nem
  promovem, protegendo o ambiente.
- **Lint de contrato (Spectral) + secret scan (gitleaks)** — qualidade de API e
  segurança verificadas em todo PR, antes do merge.
- **GHCR** — integrado ao GitHub e ao `GITHUB_TOKEN`, sem credenciais externas.
