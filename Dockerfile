FROM --platform=linux/amd64 golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM --platform=linux/amd64 alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /app/api
COPY data/normalization.json data/mcc_risk.json /app/data/
ENV DATA_DIR=/app/data
ENV REFERENCE_PATH=/app/data/references.rbin
EXPOSE 8080
ENTRYPOINT ["/app/api"]
