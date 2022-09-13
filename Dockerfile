FROM golang:1.18

ENV GOPROXY="https://goproxy.cn"

WORKDIR /go/src/github.com/mykube-run/keel/

COPY . .

RUN go mod download && ls -alh

CMD go test ./...
