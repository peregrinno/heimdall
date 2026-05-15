.PHONY: default up genrefs genivf gendata

default: up

genrefs:
	go run ./cmd/genrefs -in ./data/references.json.gz -out ./data/references.rbin

genivf:
	go run ./cmd/genivf -rbin ./data/references.rbin -out ./data/references.ivf -lists 512 -iter 12

genivf-hq:
	go run ./cmd/genivf -rbin ./data/references.rbin -out ./data/references.ivf -lists 2048 -iter 15 -workers 8

gendata: genrefs genivf

up:
	docker compose down
	docker compose build
	docker compose up -d
