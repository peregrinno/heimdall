# Observabilidade e próximos passos

Rinha de Backend 2026 — **só o que ainda falta** no produto (o restante já está coberto no código: logs JSON, `GET /metrics` com histogramas de handler e KNN, contadores 2xx/4xx/5xx sem alta cardinalidade, corpo de `POST /fraud-score` fora de log).

---

## Performance e p99 (gargalo provável)

Com **~3 milhões** de vetores de referência, o gargalo tende a ser o **scan linear** no modo exato.

**Já no código**

- **Scan exato** (padrão `KNN_MODE=exact` ou vazio): partição paralela do `.rbin`, leitura compacta por linha, buffer único por worker. Benchmark: `go test ./internal/knn -bench=FraudFractionRBin_500k` (3M opcional: `HEIMDALL_BENCH_HEAVY=1`).
- **IVF + re-ranking exato** (opcional `KNN_MODE=ivf`): k-means offline gera `references.ivf`; em cada request busca-se as `KNN_NPROBE` listas mais próximas aos centroides, re-calcula distância euclidiana exata só nos candidatos e tira os 5 vizinhos. Se o conjunto de candidatos passar de `KNN_IVF_MAX_CANDIDATES` ou for pequeno demais, cai-se no **scan exato** automático.

Gerar o índice (exemplo):

```bash
go run ./cmd/genivf -rbin ./data/references.rbin -out ./data/references.ivf -lists 512 -iter 20
```

Variáveis de ambiente relevantes: `KNN_MODE`, `REFERENCE_IVF_PATH` (opcional; padrão: mesmo prefixo do `.rbin` com `.ivf`), `KNN_NPROBE`, `KNN_IVF_MAX_CANDIDATES`.

---

## O que ainda depende de você / do desafio

- **Afinar** `lists` / `KNN_NPROBE` / `KNN_IVF_MAX_CANDIDATES` para recall vs latência (trade-off).
- **Validar com o dataset e os testes oficiais** da Rinha sempre que `KNN_MODE=ivf` estiver ligado em ambiente de pontuação: o IVF pode mudar quais 5 vizinhos entram em relação ao brute force global, logo o `fraud_score` pode divergir.

---

## Referências úteis

- Repositório do desafio: [rinha-de-backend-2026](https://github.com/zanfranceschi/rinha-de-backend-2026)
- Documentação de avaliação e testes oficiais: `docs/br/AVALIACAO.md` e pasta `test/` no repositório acima.
