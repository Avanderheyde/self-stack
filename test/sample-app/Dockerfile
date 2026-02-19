FROM golang:1.22-alpine
WORKDIR /app
COPY main.go .
RUN go build -o server main.go
CMD ["./server"]
