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

- Com **vários núcleos** visíveis para o Go, o KNN em `.rbin` usa **partição + merge exato** (cada faixa devolve os 5 melhores; o top-5 global está na união).
- **`KNN_WORKERS`**: número de goroutines de partição (default: `GOMAXPROCS`, máx. 16). O `docker-compose.yml` define **`KNN_WORKERS: "8"`** nas APIs — ajuste conforme sua máquina.
- Se o processo enxergar **apenas 1 CPU** (`GOMAXPROCS=1`), o caminho permanece **serial** e tende a **estourar 2 s** sob carga alta. Opções: mais CPU no compose (teste local), API **fora** do Docker com mais núcleos, ou **índice vetorial (ANN)** + re-ranking (ver `roadmap/observabilidade-e-proximos-passos.md`).

---

## CI (GitHub Actions)

No push e em pull requests para `main`, o workflow `.github/workflows/ci.yml` executa `go vet` e `go test` em **linux/amd64** (alinhado ao ambiente da submissão).

---

## Teste de carga oficial (Rinha)

O repositório do desafio contém scripts e cenários (`run.sh`, pasta `test/`). Use-os quando quiser medir **p99** e pontuação real; isso é independente dos testes `go test` deste módulo.
