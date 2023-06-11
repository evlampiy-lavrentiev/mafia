FROM golang:latest

WORKDIR /app


COPY main.go .
COPY go.mod .
COPY go.sum .
COPY proto ./proto
COPY server ./server
COPY client ./client

RUN go build -o mafia .

CMD ["./mafia", "--mode=server"]
