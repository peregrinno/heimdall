# Operação no Windows (PowerShell)

Todos os comandos abaixo são para **PowerShell**, executados a partir da raiz do
repositório `heimdall/`. Não dependem de `make`.

Sumário:

1. [Pré-requisitos](#1-pré-requisitos)
2. [Gerar dataset (uma vez)](#2-gerar-dataset-uma-vez)
3. [Ciclo de desenvolvimento local](#3-ciclo-de-desenvolvimento-local)
4. [Regenerar o perfil PGO](#4-regenerar-o-perfil-pgo)
5. [Publicar imagem no Docker Hub](#5-publicar-imagem-no-docker-hub)
6. [Atualizar a branch `submission`](#6-atualizar-a-branch-submission)
7. [Validar localmente com k6 oficial](#7-validar-localmente-com-k6-oficial)
8. [Solução de problemas frequentes](#8-solução-de-problemas-frequentes)

---

## 1. Pré-requisitos

| Ferramenta | Versão mínima | Verificar |
|-----------|---------------|-----------|
| Go | 1.23 | `go version` |
| Docker Desktop | 4.x com BuildKit | `docker version` |
| Git | qualquer | `git --version` |
| k6 (opcional) | 0.50+ | `k6 version` |

Login no Docker Hub (uma vez por máquina):

```powershell
docker login -u peregrinno
```

Variável de ambiente útil (evita o Go baixar toolchains alternativas a cada `go get`):

```powershell
$env:GOTOOLCHAIN = "local"
# para fixar permanentemente:
[Environment]::SetEnvironmentVariable("GOTOOLCHAIN", "local", "User")
```

---

## 2. Gerar dataset (uma vez)

Os ficheiros `data/references.rbin` (~192 MB) e `data/references.ivf` (~12 MB)
**não** estão versionados. Precisam ser gerados a partir de
`data/references.json.gz` (baixado do repo oficial da Rinha).

```powershell
# Converter JSON → rbin (alguns minutos)
go run ./cmd/genrefs -in .\data\references.json.gz -out .\data\references.rbin

# Treinar IVF 2048 listas (~30-60 min, paralelo)
go run ./cmd/genivf -rbin .\data\references.rbin -out .\data\references.ivf `
    -lists 2048 -iter 15 -workers 8
```

Quando treinar com **512 listas** (mais rápido, ~5-15 min):

```powershell
go run ./cmd/genivf -rbin .\data\references.rbin -out .\data\references.ivf `
    -lists 512 -iter 12
```

Verificar:

```powershell
"{0:N0} bytes rbin" -f (Get-Item data/references.rbin).Length
"{0:N0} bytes ivf"  -f (Get-Item data/references.ivf).Length
```

Esperado: ~`192.000.196 bytes rbin` e ~`12.122.948 bytes ivf` (2048 listas).

---

## 3. Ciclo de desenvolvimento local

### 3.1 Subir a stack (com warmup automático)

```powershell
docker compose down -v
docker compose up --build -d
```

Acompanhar logs:

```powershell
docker compose logs -f --tail=20 api-1 lb warmup
```

Procurar nos logs:

```text
warmup mmap concluído                       elapsed_ms=… (pre-touch interno)
referências prontas                         n=3000000 knn_effective=ivf
GC automático desligado; GC periódico ativo interval_s=5
warmup: 120 requests concluídas             (do container warmup)
```

### 3.2 Smoke test

```powershell
curl http://localhost:9999/ready
curl -X POST http://localhost:9999/fraud-score `
    -H "Content-Type: application/json" `
    -d (Get-Content -Raw .\scripts\smoke-payload.json)
```

(Se `scripts\smoke-payload.json` não existir, usar um payload de exemplo do
`scripts/warmup.sh`.)

### 3.3 Derrubar

```powershell
docker compose down -v
```

---

## 4. Regenerar o perfil PGO

O perfil em `cmd/api/default.pgo` é consumido pelo build de produção
(`Dockerfile.hub` → `-pgo=auto`). Regerar **quando o hot path muda**
(arquivos `internal/knn/*.go`, `internal/app/index.go`, `internal/httpserver/server.go`).

### Atalho (recomendado)

```powershell
.\scripts\gen-pgo.ps1                 # default 5s
.\scripts\gen-pgo.ps1 -BenchTime 10s  # mais amostras
```

### Manual

```powershell
$env:GOTOOLCHAIN = "local"
go test .\internal\knn\ -run="^$" -bench=BenchmarkFraudFractionRBinIVF `
    -benchtime=5s -outputdir=cmd/api -cpuprofile=default.pgo

# O Go grava como "default" no Windows; renomear para .pgo:
if (Test-Path cmd\api\default) { Move-Item cmd\api\default cmd\api\default.pgo -Force }

"{0:N0} bytes pgo" -f (Get-Item cmd/api/default.pgo).Length

# Validar que o build aceita o perfil
go build -trimpath -pgo=auto -ldflags="-s -w" -o nul .\cmd\api
```

Esperado: arquivo de ~8-15 KB e build sem erro. Comitar `cmd/api/default.pgo`.

---

## 5. Publicar imagem no Docker Hub

A imagem `peregrinno/heimdall:latest` **precisa** conter `references.rbin` e
`references.ivf` (a Rinha não monta volumes externos). Build feito com
`Dockerfile.hub`.

### 5.1 Atalho (recomendado)

```powershell
.\scripts\build-push.ps1
# ou tag personalizada:
.\scripts\build-push.ps1 -Image peregrinno/heimdall:v2
```

O script:
- valida pré-requisitos (rbin, ivf, default.pgo),
- builda `--no-cache` com `GOAMD64=v3 -pgo=auto`,
- faz push,
- imprime o **digest** no final.

### 5.2 Manual

```powershell
docker build --no-cache --platform linux/amd64 -f Dockerfile.hub `
    --build-arg GIT_SHA=$(git rev-parse --short HEAD) `
    -t peregrinno/heimdall:latest .

docker push peregrinno/heimdall:latest

$digest = (docker image inspect peregrinno/heimdall:latest `
    --format "{{(index .RepoDigests 0)}}")
$digest = ($digest -split "@")[1]
Write-Host "Digest: $digest"
```

Tempo típico: **build 1-3 min**, **push 30 s - 2 min** (200 MB de layers).

### 5.3 Confirmar que a imagem subiu correta

```powershell
docker run --rm --platform linux/amd64 `
    -e LISTEN=:8080 -e MIN_REFERENCES=0 `
    peregrinno/heimdall:latest 2>&1 | Select-Object -First 5
```

Esperado:

```text
{"msg":"heimdall","version":"<commit_curto>"}
{"msg":"carregando referências","path":"/app/data/references.rbin"}
{"msg":"referências prontas","n":3000000,"knn_effective":"ivf"}
```

Confirma que: (a) o binário foi recompilado (`version` igual ao commit atual),
(b) tem 3 M vetores na imagem e (c) IVF carrega.

---

## 6. Atualizar a branch `submission`

A `submission` usa **digest fixo** (`image: peregrinno/heimdall@sha256:...`) para
evitar `:latest` ambíguo no runner da Rinha. Trabalhamos via `git worktree` para
não poluir a `main`.

### 6.1 Criar worktree (uma vez)

```powershell
git worktree add ..\heimdall-submission submission
```

Para listar/remover:

```powershell
git worktree list
git worktree remove ..\heimdall-submission   # quando terminar
```

### 6.2 Atalho (recomendado)

```powershell
.\scripts\sync-submission.ps1
```

O script lê o digest atual de `peregrinno/heimdall:latest` (já no `docker image inspect`
após o push), troca a linha `image:` no `docker-compose.yml` da submission,
copia `scripts/warmup.sh` e `deploy/haproxy.cfg`, e faz commit + push.

### 6.3 Manual

Na pasta `..\heimdall-submission\`:

1. Editar `docker-compose.yml`: trocar o `sha256:...` antigo pelo novo.
2. Copiar ficheiros suportes recém-modificados na `main`:

   ```powershell
   cd ..\heimdall-submission
   Copy-Item ..\heimdall\scripts\warmup.sh   .\scripts\warmup.sh   -Force
   Copy-Item ..\heimdall\deploy\haproxy.cfg  .\deploy\haproxy.cfg  -Force
   ```

3. Commit + push:

   ```powershell
   git add .
   git commit -m "submission: pin image @ <commit_curto> + tunings"
   git push origin submission
   cd ..\heimdall
   ```

### 6.4 Automatizar só o pin de digest (opcional)

Há um workflow `.github/workflows/sync-submission.yml` que, após o CI verde na
`main`, lê o digest do Docker Hub e atualiza só a linha `image:` na
`submission`. Mudanças em `haproxy.cfg`/`scripts/` continuam manuais.

Para forçar manualmente: **GitHub → Actions → Sync submission digest → Run workflow**,
preenche o campo `image_digest` se quiser fixar um específico.

---

## 7. Validar localmente com k6 oficial

Pré-requisito: ter o repo oficial da Rinha clonado fora do `heimdall/`:

```powershell
cd ..
git clone https://github.com/zanfranceschi/rinha-de-backend-2026.git
cd rinha-de-backend-2026
```

Subir a stack `heimdall` e rodar o teste:

```powershell
# em outro PowerShell
cd c:\Users\jose_\Documents\PROJETOS_CODE_2\heimdall
docker compose up --build -d
docker compose logs -f warmup    # aguarda "120 requests concluídas"

# no PowerShell da rinha
cd c:\Users\jose_\Documents\PROJETOS_CODE_2\rinha-de-backend-2026
k6 run .\test\test.js
```

O resultado JSON é impresso no fim. `final_score` é o que interessa.

---

## 8. Variáveis de ambiente de tuning

Estas envs controlam o comportamento do binário em produção. Os defaults em
`docker-compose.yml` já estão calibrados para o ambiente da Rinha (0.45 CPU,
159 MB de RAM por API). Alterar só se for medir.

### 8.1 KNN

| Variável | Default | Função |
|----------|---------|--------|
| `KNN_MODE` | `ivf` | `ivf` força IVF; `exact` força scan completo; `auto` cai para exato se `.ivf` ausente |
| `KNN_NPROBE` | `12` | Número de listas IVF percorridas por query. ↑ = mais precisão, mais latência |
| `KNN_IVF_MAX_CANDIDATES` | `4000` | Candidatos máximos rerankeados após IVF. ↑ = mais precisão, mais alloc |

### 8.2 Runtime Go

| Variável | Default | Função |
|----------|---------|--------|
| `GOMAXPROCS` | `2` | Threads do runtime. Para 0.45 CPU, 2 evita travas de scheduler |
| `GOGC` | `200` | % de heap antes de GC automático. Só usado se `HEIMDALL_DISABLE_GC=0` |
| `HEIMDALL_MEM_LIMIT_BYTES` | `120000000` | `debug.SetMemoryLimit`. Mantém heap abaixo do limite do container |
| `HEIMDALL_DISABLE_GC` | `1` | Se `1`, desliga GC automático e usa GC periódico manual |
| `HEIMDALL_GC_INTERVAL_SEC` | `5` | Intervalo do `runtime.GC()` periódico quando o automático está off |

### 8.3 Load shedding (Etapa 4)

| Variável | Default | Função |
|----------|---------|--------|
| `HEIMDALL_SHED_SLOTS` | `32` | Requests simultâneas aceitas em `/fraud-score`. `0` = sem limite |
| `HEIMDALL_SHED_TIMEOUT_MS` | `3` | Tempo máximo aguardando slot livre antes de responder `503` |

Por que isso ajuda: sob rajada (k6 sobe para 100 VUs em < 1 s), preferimos
devolver `503` para o excesso e manter o p99 baixo nos requests aceitos. O k6
oficial só corta o teste se a taxa de erro passar de 5%, então até ~5 req/s de
shed é gratuito em score.

### 8.4 HAProxy (Etapa 3)

O `deploy/haproxy.cfg` está em `mode tcp` (sem parsing HTTP). Não tem env
direta, mas se quiser voltar para HTTP em debug, troque o `mode tcp` para
`mode http` e adicione de volta `option http-keep-alive` no `defaults`.

---

## 9. Solução de problemas frequentes

### 9.1 “/ready not ready yet” no CI da Rinha

Causa típica: imagem antiga no Docker Hub (binário velho com bug de IVF) ou
`KNN_MODE=ivf` em imagem sem `.ivf`.

Solução: re-fazer **5.1** (rebuild `--no-cache`) e **6.2** (pinar novo digest na
`submission`).

### 9.2 `docker push` reporta “Layer already exists”

Significa que você está empurrando uma **imagem existente** (sem rebuild). É
inofensivo, mas não atualiza o digest. Para forçar imagem nova: rebuilda com
`--no-cache` antes do push.

### 9.3 Layer do `/out/api` mais antigo que dos dados

`docker history peregrinno/heimdall:latest` mostra as layers com timestamp.
Se a layer do binário for mais antiga que a do `references.rbin`, o BuildKit
cacheou um `go build` anterior. Solução: `--no-cache` no `docker build`.

### 9.4 `go: golang.org/x/sys@vX.Y requires go >= 1.25`

Acontece quando um `go get` puxa versão nova de `x/sys` que exige Go mais novo
que o `go.mod`. Solução:

```powershell
go get golang.org/x/sys@v0.30.0
```

### 9.5 `cannot use -cpuprofile flag with multiple packages`

Acontece com `go test ./... -cpuprofile=...`. Solução: rodar o `go test` num
único pacote explícito, como mostrado em **§4**.

### 9.6 O perfil PGO é salvo como `default` (sem extensão) no Windows

Bug de interação PowerShell × `go test -outputdir`. Mover manualmente:

```powershell
if (Test-Path cmd\api\default) { Move-Item cmd\api\default cmd\api\default.pgo -Force }
```

### 9.7 `docker compose` não acha o `scripts/warmup.sh` na `submission`

A branch `submission` precisa do script copiado (ver **§6.2** passo 2).

### 9.8 `warmup-1 | /warmup.sh: set: line 7: illegal option -`

Causa: Docker Desktop no Windows às vezes injeta CRLF em **bind mounts de
arquivo único**, mesmo quando o arquivo no disco está em LF. O `\r` final
faz o shell ver `set -eu\r` como flag inválida.

Corrigido permanentemente por três medidas combinadas (já no repo):

1. `.gitattributes` força `*.sh text eol=lf`.
2. `docker-compose.yml` monta o **diretório** `./scripts:/scripts:ro` em vez
   do arquivo único `./scripts/warmup.sh:/warmup.sh:ro`.
3. O entrypoint do warmup é `["/bin/sh", "-e", "/scripts/warmup.sh"]` — o
   `-e` na linha de comando dispensa o `set -e` dentro do script.

Se um script novo aparecer com esse erro, garanta que:
- ele está coberto por algum padrão em `.gitattributes`,
- está sendo lido de um bind mount de diretório,
- e roda com `sh -e` (ou tem `set -e` em LF puro).

---

## Fluxo completo de submissão (após mudança no hot path)

Versão curta com os 3 scripts:

```powershell
# 1. Código alterado + commit na main
git add .
git commit -m "perf: ..."

# 2. Regenerar PGO (se mexeu no hot path)
.\scripts\gen-pgo.ps1
git add cmd/api/default.pgo
git commit --amend --no-edit   # ou novo commit

# 3. Build + push da imagem (imprime o digest)
.\scripts\build-push.ps1

# 4. Sincronizar branch submission com o novo digest
.\scripts\sync-submission.ps1

# 5. Push da main
git push origin main
```

A Rinha agenda novo teste quando você comenta `rinha/test` na issue da
submissão; consulta o resultado em ~5-10 min.

### Versão expandida (sem scripts)

```powershell
# 1. Código alterado + commit na main
git add .
git commit -m "perf: ..."

# 2. Regenerar PGO
$env:GOTOOLCHAIN = "local"
go test .\internal\knn\ -run="^$" -bench=BenchmarkFraudFractionRBinIVF `
    -benchtime=5s -outputdir=cmd/api -cpuprofile=default.pgo
if (Test-Path cmd\api\default) { Move-Item cmd\api\default cmd\api\default.pgo -Force }
go build -trimpath -pgo=auto -ldflags="-s -w" -o nul .\cmd\api
git add cmd/api/default.pgo
git commit -m "perf: regenerate PGO profile"

# 3. Build + push imagem
docker build --no-cache --platform linux/amd64 -f Dockerfile.hub `
    --build-arg GIT_SHA=$(git rev-parse --short HEAD) `
    -t peregrinno/heimdall:latest .
docker push peregrinno/heimdall:latest

# 4. Capturar digest
$digest = (docker image inspect peregrinno/heimdall:latest `
    --format "{{(index .RepoDigests 0)}}")
$digest = ($digest -split "@")[1]
Write-Host "Digest: $digest"

# 5. Atualizar submission
cd ..\heimdall-submission
# editar docker-compose.yml: trocar @sha256:... pelo $digest
Copy-Item ..\heimdall\scripts\warmup.sh   .\scripts\warmup.sh   -Force
Copy-Item ..\heimdall\deploy\haproxy.cfg  .\deploy\haproxy.cfg  -Force
git add .
git commit -m "submission: pin @ $($digest.Substring(7,12))"
git push origin submission

# 6. Push da main
cd ..\heimdall
git push origin main
```
