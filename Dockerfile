FROM node:26-alpine AS web-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS go-builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /src/cmd/server/web/dist ./cmd/server/web/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/sakura-home ./cmd/server

FROM alpine:3.22 AS app
WORKDIR /app
RUN addgroup -S -g 10001 sakura \
    && adduser -S -D -H -u 10001 -G sakura sakura \
    && install -d -o sakura -g sakura /app/data/uploads
COPY --from=go-builder /out/sakura-home /app/sakura-home
USER 10001:10001
EXPOSE 13888
ENTRYPOINT ["/app/sakura-home"]
