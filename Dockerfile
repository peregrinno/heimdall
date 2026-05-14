FROM --platform=linux/amd64 golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/genrefs ./cmd/genrefs
RUN /out/genrefs -in /src/data/example-references.json -out /out/default-references.rbin

FROM --platform=linux/amd64 alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /app/api
COPY data/normalization.json data/mcc_risk.json /app/data/
COPY --from=build /out/default-references.rbin /app/data/references.rbin
ENV DATA_DIR=/app/data
ENV REFERENCE_PATH=/app/data/references.rbin
EXPOSE 8080
ENTRYPOINT ["/app/api"]
