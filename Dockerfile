FROM golang:1.17 AS builder

ENV GOOS=linux
ENV GOARCH=386

WORKDIR /work

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY main.go .
COPY pkg ./pkg
RUN go build -o ./elvanto-oversikt .

FROM alpine:3.14

COPY --from=builder /work/elvanto-oversikt .
COPY views ./views

ENV GIN_MODE=release

ENTRYPOINT ["./elvanto-oversikt"]
