FROM golang:1.26-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags='-s -w' -o /out/sakura-home ./cmd/server

FROM alpine:3.21 AS app
WORKDIR /app
RUN addgroup -S -g 10001 sakura \
    && adduser -S -D -H -u 10001 -G sakura sakura \
    && install -d -o sakura -g sakura /app/data/uploads
COPY --from=builder /out/sakura-home /app/sakura-home
USER 10001:10001
EXPOSE 13888
ENTRYPOINT ["/app/sakura-home"]
