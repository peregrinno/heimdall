# Observabilidade e próximos passos

Rinha de Backend 2026 — **só o que ainda falta** no produto (o restante já está coberto no código: logs JSON, `GET /metrics` com histogramas de handler e KNN, contadores 2xx/4xx/5xx sem alta cardinalidade, corpo de `POST /fraud-score` fora de log).

---

## Performance e p99 (gargalo provável)

Com **~3 milhões** de vetores de referência, o gargalo tende a ser o **scan linear** (uma passagem completa por requisição no KNN exato).

**Próximo salto possível:**

1. **ANN (busca aproximada)** + **re-ranking** com distância exata em um subconjunto pequeno de candidatos (ex.: top 200–2000 da ANN → recalcula euclidiana e escolhe os 5 definitivos).
2. **Paralelismo** do scan com **merge correto** dos vizinhos (cuidado: empates em distância e ordem de atualização podem alterar quais 5 pontos entram; a prova pode assumir KNN euclidiano determinístico como no enunciado).

Qualquer atalho que mude os 5 vizinhos em relação ao brute force exato pode impactar **`fraud_score`** e a pontuação de detecção — valide com o dataset oficial e com os testes da Rinha.

---

## Referências úteis

- Repositório do desafio: [rinha-de-backend-2026](https://github.com/zanfranceschi/rinha-de-backend-2026)
- Documentação de avaliação e testes oficiais: `docs/br/AVALIACAO.md` e pasta `test/` no repositório acima.
