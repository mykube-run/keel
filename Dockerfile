FROM bitnamilegacy/golang:1.18.9-debian-11-r9

ENV GOPROXY="https://goproxy.cn"

WORKDIR /go/src/github.com/mykube-run/keel/

COPY . .

RUN go mod download && ls -alh

ARG BUILD_TARGET

RUN mkdir -p bin/ && \
    go build -o ./bin/app $BUILD_TARGET && \
    chmod +x ./bin/app

CMD ./bin/app
