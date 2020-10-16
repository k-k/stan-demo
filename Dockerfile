FROM golang:1.12

WORKDIR /app

ENV GO111MODULE=on

RUN go get -u github.com/rakyll/gotest

ADD ./go.mod .
ADD ./go.sum .
RUN go mod download
