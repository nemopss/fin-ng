# Этап сборки
FROM --platform=linux/arm64 golang:1.24.4 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o app ./main.go

# Итоговый образ
FROM --platform=linux/arm64 alpine:latest

# Установка совместимости с glibc, если требуется
RUN apk add --no-cache libc6-compat

WORKDIR /root/

COPY --from=builder /app/app /root/app

COPY .env /root/.env

EXPOSE 8080

CMD ["/root/app"]
