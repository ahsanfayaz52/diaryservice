FROM golang:1.23-alpine


WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN apk add --no-cache gcc musl-dev

RUN CGO_ENABLED=1 go build -o go-diary ./cmd/server

EXPOSE 8080

CMD ["./go-diary"]
