# ----------------------------
# 1) BUILD STAGE
# ----------------------------
    FROM golang:1.23-alpine AS builder
    RUN apk add --no-cache git gcc musl-dev
    WORKDIR /src
    COPY go.mod go.sum ./
    RUN go mod download
    COPY . .
    RUN mkdir -p pb_migrations
    RUN CGO_ENABLED=0 GOOS=linux go build -o pocketbase-app .
    
    # ----------------------------
    # 2) RUNTIME STAGE
    # ----------------------------
    FROM alpine:latest
    RUN apk add --no-cache ca-certificates
    WORKDIR /app
    COPY --from=builder /src/pocketbase-app .
    COPY --from=builder /src/pb_hooks    ./pb_hooks
    COPY --from=builder /src/pb_migrations ./pb_migrations
    COPY --from=builder /src/pb_public   ./pb_public
    RUN mkdir -p /mnt/data && ln -s /mnt/data /app/pb_data
    
    ARG HTTP_PORT=8085
    # Exportamos HTTP_PORT para flags y PORT para el código Go
    ENV HTTP_PORT=${HTTP_PORT} \
        PORT=${HTTP_PORT}
    
    EXPOSE ${HTTP_PORT}
    
    # Usamos shell form para que expanda ${PORT}
    ENTRYPOINT ["sh", "-c", "./pocketbase-app serve --http=0.0.0.0:${PORT} --dir=/app/pb_data"]
    