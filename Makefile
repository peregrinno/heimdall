.PHONY: default up genrefs genivf gendata docker-hub-build docker-hub-push

IMAGE ?= peregrinno/heimdall:latest

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

# Imagem para submissão (Rinha): embute references.rbin + references.ivf (~200MB).
# Requer data/references.rbin e data/references.ivf (make gendata).
docker-hub-build:
	docker build --no-cache --platform linux/amd64 -f Dockerfile.hub \
		--build-arg GIT_SHA=$$(git rev-parse --short HEAD) \
		-t $(IMAGE) .

docker-hub-push: docker-hub-build
	docker push $(IMAGE)
	@echo "Digest (use na branch submission para fixar a imagem):"
	@docker image inspect $(IMAGE) --format '{{index .RepoDigests 0}}'
