# Heimdall

Backend de **detecção de fraude por busca vetorial** para a [Rinha de Backend 2026](https://github.com/zanfranceschi/rinha-de-backend-2026) (desafio oficial, regras e massa de teste no repositório linkado).

---

## O que o serviço faz

- Expõe **`GET /ready`** (2xx quando o índice de referência está carregado) e **`POST /fraud-score`** (payload da transação → decisão).
- Para cada transação: **vetor de 14 dimensões** (normalização conforme o doc da Rinha, sentinela `-1` quando não há `last_transaction`) → **5 vizinhos mais próximos** no dataset de referência (distância euclidiana) → **`fraud_score` = fração de fraudes entre esses 5** → **`approved = fraud_score < 0.6`**.

A especificação exata dos campos e fórmulas está em [docs/br no repositório da Rinha](https://github.com/zanfranceschi/rinha-de-backend-2026/tree/main/docs/br).

---

## Estratégia técnica (porquê assim)

| Escolha | Motivo |
|--------|--------|
| **Dataset em `.rbin` + mmap** | O `references.json.gz` oficial descompacta em centenas de MB em JSON; em **binário compacto** (`float32` por dimensão + rótulo) o ficheiro fica menor e pode ser **mapeado na memória** sem deserializar milhões de objetos para o heap do Go. |
| **KNN exato em CPU** | A avaliação compara com rótulos obtidos por **k-NN exato (k=5, euclidiana)** sobre as referências. Mantemos o mesmo critério de distância para não divergir da grelha de testes. |
| **Partição paralela + merge** | O scan linear em ~3M pontos é pesado. Com **vários núcleos**, o vetor é dividido em faixas; cada faixa calcula os **5** melhores vizinhos locais; o **top-5 global** está sempre contido na **união** desses conjuntos (merge por ordenação dos candidatos). É **exatamente equivalente** ao brute force num único fio, com menos tempo de parede quando há CPU paralela. |
| **`KNN_WORKERS` (env)** | Goroutines de partição no KNN paralelo (opcional; por defeito derivado de `GOMAXPROCS`, máx. 8). No Docker usa valores baixos (ex.: `2`) quando a CPU por réplica é limitada. |

**Limitação honesta:** por requisição o trabalho continua **O(N)** em relação ao número de referências. Sob **taxas muito altas** (ex.: script k6 da Rinha a ~900 req/s), o sistema pode **acumular fila** e estourar o **timeout de 2001 ms** do cliente — isso é esperado com scan completo sem índice aproximado. O próximo salto de performance é **ANN + re-ranking** (ou outro índice), descrito em `roadmap/observabilidade-e-proximos-passos.md`.

---

## Arquitetura de deploy (requisito da Rinha)

- **HAProxy** na porta **9999** (round-robin; só encaminha HTTP, sem lógica de negócio). Liga às réplicas por **Unix sockets** num volume `tmpfs` partilhado (`lb_sockets` → `/run/sockets`). Variante TCP: `deploy/nginx.conf` com *upstream keepalive* e `LISTEN=:8080` nas APIs.
- **Duas réplicas** da API Go; no compose por defeito **`LISTEN=unix:/run/sockets/api-N.sock`** (sem porta TCP entre LB e API).
- Limites no `docker-compose.yml`: **até 1 CPU e 350 MB no total** (soma dos serviços), conforme [ARQUITETURA.md](https://github.com/zanfranceschi/rinha-de-backend-2026/blob/main/docs/br/ARQUITETURA.md).

No código: organização em camadas (`internal/domain`, `internal/vector`, `internal/knn`, `internal/reference`, `internal/app`, `internal/httpserver`) para manter o núcleo testável e o HTTP fino.

---

## Estrutura do repositório

| Caminho | Função |
|---------|--------|
| `cmd/api` | Servidor HTTP |
| `cmd/genrefs` | Converte `references.json.gz` → `references.rbin` |
| `internal/vector` | Vetorização + testes alinhados aos exemplos oficiais |
| `internal/knn` | KNN em RAM ou sobre mmap `.rbin` |
| `internal/reference` | Loader JSON, formato `.rbin`, mmap |
| `deploy/haproxy.cfg` | LB na porta 9999 (round-robin, keep-alive para as APIs) |
| `deploy/nginx.conf` | Variante Nginx (upstream keepalive) |
| `data/` | `normalization.json`, `mcc_risk.json`, `references.json.gz` / `references.rbin` (não versionar o binário gigante se não quiseres) |
| `roadmap/` | Observabilidade, próximos passos, ambiente de testes |
| `scripts/` | `test.ps1`, `smoke.ps1`, `test.sh` |

---

## Arranque rápido

### 1. Dataset e binário

1. Obtém `references.json.gz` do repositório da Rinha em `data/`.
2. Gera o binário:

```powershell
go run ./cmd/genrefs -in .\data\references.json.gz -out .\data\references.rbin
```

### 2. Docker

```powershell
docker compose up -d --build
```

- API pública: `http://localhost:9999`
- Smoke: `.\scripts\smoke.ps1`

### 3. Testes Go

```powershell
.\scripts\test.ps1
```

---

## Variáveis de ambiente (API)

| Variável | Significado |
|----------|-------------|
| `LISTEN` | Endereço de escuta: **`:8080`** (TCP) ou **`unix:/caminho/ficheiro.sock`** (socket Unix; `chmod` 0666 para o HAProxy alpine ligar como utilizador `haproxy`) |
| `DATA_DIR` | Pasta com `normalization.json` e `mcc_risk.json` |
| `REFERENCE_PATH` | Caminho para `references.rbin` **ou** `references.json(.gz)` |
| `KNN_WORKERS` | Número de workers do KNN particionado (opcional) |

---

## Documentação interna

- [`roadmap/observabilidade-e-proximos-passos.md`](roadmap/observabilidade-e-proximos-passos.md) — métricas, p99, ANN, LGPD.
- [`roadmap/ambiente-de-testes.md`](roadmap/ambiente-de-testes.md) — testes unitários, smoke, k6, timeouts.

---

## Submissão à Rinha

Tutorial passo a passo (dados, branches, PR, k6 oficial): [`docs/submissao-e-teste-de-carga.md`](docs/submissao-e-teste-de-carga.md). Regras gerais: [SUBMISSAO.md](https://github.com/zanfranceschi/rinha-de-backend-2026/blob/main/docs/br/SUBMISSAO.md).

---

## Licença

Ver ficheiro `LICENSE` na raiz do projeto.
