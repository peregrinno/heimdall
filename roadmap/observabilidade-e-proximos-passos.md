# Observabilidade e próximos passos

Documento de orientação para evolução do Heimdall (Rinha de Backend 2026).

---

## Observabilidade (estado atual)

- **Logs estruturados em JSON** (`slog`) no startup e em erros de decodificação HTTP continuam sendo a base.
- Próximo passo natural: **métricas** (ex.: Prometheus) com histograma de latência do handler `POST /fraud-score`, contadores de `4xx`/`5xx` e tempo interno do passo de KNN (sem expor payload).

---

## Performance e p99 (gargalo provável)

Com **~3 milhões** de vetores de referência, o gargalo tende a ser o **scan linear** (uma passagem completa por requisição no KNN exato).

**Próximo salto possível:**

1. **ANN (busca aproximada)** + **re-ranking** com distância exata em um subconjunto pequeno de candidatos (ex.: top 200–2000 da ANN → recalcula euclidiana e escolhe os 5 definitivos).
2. **Paralelismo** do scan com **merge correto** dos vizinhos (cuidado: empates em distância e ordem de atualização podem alterar quais 5 pontos entram; a prova pode assumir KNN euclidiano determinístico como no enunciado).

Qualquer atalho que mude os 5 vizinhos em relação ao brute force exato pode impactar **`fraud_score`** e a pontuação de detecção — valide com o dataset oficial e com os testes da Rinha.

---

## LGPD e dados sensíveis

- **Não logar o corpo** de `POST /fraud-score` (payload de transação contém dados que em produção exigiriam base legal, minimização e retenção definidas).
- Em **métricas**, evitar **labels de alta cardinalidade** (ex.: `transaction_id`, `merchant_id` em cada série) — isso explode cardinalidade no backend de métricas e pode reter dados identificáveis além do necessário.

---

## Referências úteis

- Repositório do desafio: [rinha-de-backend-2026](https://github.com/zanfranceschi/rinha-de-backend-2026)
- Documentação de avaliação e testes oficiais: `docs/br/AVALIACAO.md` e pasta `test/` no repositório acima.
