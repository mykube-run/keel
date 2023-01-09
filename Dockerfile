FROM bitnami/golang:1.18.9-debian-11-r9

ENV GOPROXY="https://goproxy.cn"

WORKDIR /go/src/github.com/mykube-run/keel/

COPY . .

RUN go mod download && ls -alh

RUN go get github.com/mykube-run/keel/tests/testkit/worker

CMD sh tests/test.sh
