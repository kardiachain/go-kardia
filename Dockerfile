FROM golang:1.13-stretch
RUN mkdir -p "$GOPATH/src/github.com/kardiachain/go-kardia"
WORKDIR /go/src/github.com/kardiachain/go-kardia
RUN apt-get update && apt-get install -y libzmq3-dev
ADD . .
WORKDIR /go/src/github.com/kardiachain/go-kardia/cmd
RUN go install
WORKDIR /go/src/github.com/kardiachain/go-kardia/tool/pump
RUN go install
WORKDIR /go/src/github.com/kardiachain/go-kardia/dualnode/eth/eth_client
RUN go install
WORKDIR /go/bin
ENTRYPOINT ["./cmd"]
