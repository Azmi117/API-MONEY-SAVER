# STAGE 1: Builder & Tester
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache tesseract-ocr tesseract-ocr-data-ind tesseract-ocr-data-eng
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main ./cmd/api/main.go
RUN go build -o migrate ./cmd/migrate/main.go

# STAGE 2: Runner
FROM alpine:latest
RUN apk add --no-cache tesseract-ocr tesseract-ocr-data-ind tesseract-ocr-data-eng tzdata
WORKDIR /root/
COPY --from=builder /app/main .
COPY --from=builder /app/migrate .
COPY --from=builder /app/docs ./docs
EXPOSE 8080
CMD ["./main"]