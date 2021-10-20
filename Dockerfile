FROM golang:1.17 AS builder

ENV GOOS=linux
ENV GOARCH=386

WORKDIR /work

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY main.go models.go ./
RUN go build -o ./elvanto-oversikt .

FROM alpine:3.14

COPY --from=builder /work/elvanto-oversikt .
COPY ./template.html .

ENV GIN_MODE=release

ENTRYPOINT ["./elvanto-oversikt"]
