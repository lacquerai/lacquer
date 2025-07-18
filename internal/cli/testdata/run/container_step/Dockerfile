FROM golang:latest

WORKDIR /app

COPY ./scripts/main.go .

RUN go build -o main main.go

CMD ["./main"]