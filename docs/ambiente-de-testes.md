# Ambiente de testes

Como executar testes automatizados e smoke checks do projeto Heimdall no seu ambiente.

---

## Pré-requisitos

| Ferramenta | Uso |
|--------------|-----|
| **Go 1.23+** | `go test`, `go run ./cmd/genrefs`, `go run ./cmd/api` |
| **Docker / Docker Compose** | Testes de integração com LB (HAProxy ou Nginx) + 2 APIs (opcional para CI unitário) |

---

## Testes unitários (rápido)

Na raiz do repositório:

### PowerShell (Windows)

```powershell
.\scripts\test.ps1
```

### Bash / Linux / macOS

```bash
chmod +x scripts/test.sh
./scripts/test.sh
```

### Manual

```bash
go vet ./...
go test ./... -count=1
```

---

## Binário de referência (`.rbin`)

Os testes de pacote **não** dependem do arquivo de 3M linhas. Para rodar a API com dados reais:

1. Coloque `references.json.gz` em `data/` (download do repositório da Rinha).
2. Gere o binário:

```powershell
go run ./cmd/genrefs -in .\data\references.json.gz -out .\data\references.rbin
```

---

## Smoke com Docker Compose

1. Garanta `data/references.rbin`, `data/normalization.json`, `data/mcc_risk.json`.
2. Suba os serviços:

```powershell
docker compose up -d --build
```

3. Valide readiness e um score:

```powershell
.\scripts\smoke.ps1
```

Ou manualmente: `GET http://localhost:9999/ready` e `POST http://localhost:9999/fraud-score` — exemplo de payload no script `scripts/smoke.ps1`.

### k6: `request timeout` (2001 ms)

O script oficial do k6 usa **timeout de 2001 ms** por requisição. O gargalo costuma ser o **KNN linear em ~3 milhões de vetores** por chamada.

Neste projeto:

- O KNN em `.rbin` faz **scan exato** em ~3M vetores (com paralelismo opcional via `KNN_WORKERS`).
- O `docker-compose.yml` de submissão usa **`GOMAXPROCS=1`** e **`KNN_WORKERS=1`** por réplica (0,45 CPU cada), alinhado ao limite da Rinha.
- Sob **~900 req/s** (script k6 oficial), o p99 pode ultrapassar **2001 ms** — veja [submissao-e-teste-de-carga.md](./submissao-e-teste-de-carga.md) para rodar o teste real e interpretar `test/results.json`.

---

## CI (GitHub Actions)

No push e em pull requests para `main`, o workflow `.github/workflows/ci.yml` executa `go vet` e `go test` em **linux/amd64** (alinhado ao ambiente da submissão).

---

## Teste de carga oficial (Rinha)

Tutorial completo (clone do repo da Rinha, k6, `results.json`, issue de prévia): **[submissao-e-teste-de-carga.md](./submissao-e-teste-de-carga.md)**.
