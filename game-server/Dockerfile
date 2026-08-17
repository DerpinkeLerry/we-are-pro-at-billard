FROM golang:1.23-alpine AS build
WORKDIR /src/game-server
COPY game-server/go.mod game-server/go.sum ./
RUN go mod download
COPY game-server .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pool-server ./cmd/pool-server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/pool-server /app/pool-server
COPY config /app/config
ENV CONFIG_DIR=/app/config
EXPOSE 8081
USER 65532:65532
ENTRYPOINT ["/app/pool-server"]
