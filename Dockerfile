FROM golang:1.12-stretch
RUN mkdir -p "$GOPATH/src/github.com/kardiachain/go-kardiamain"
WORKDIR /go/src/github.com/kardiachain/go-kardiamain
RUN apt-get update && apt-get install -y libzmq3-dev
ADD . .
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN dep ensure
WORKDIR /go/src/github.com/kardiachain/go-kardia/cmd
RUN go install
WORKDIR /go/src/github.com/kardiachain/go-kardia/tool/pump
RUN go install
WORKDIR /go/src/github.com/kardiachain/go-kardia/dualnode/eth/eth_client
RUN go install
WORKDIR /go/bin
ENTRYPOINT ["./cmd"]
