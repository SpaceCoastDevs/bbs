FROM golang:latest AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /main .

FROM alpine:latest
COPY --from=builder /main /main

EXPOSE 22
CMD ["/main ssh"]