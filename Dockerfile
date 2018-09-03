FROM golang:1.10
RUN mkdir -p "$GOPATH/src/github.com/kardiachain/go-kardia"
WORKDIR /go/src/github.com/kardiachain/go-kardia
RUN apt-get update && apt-get install -y vim
ADD . .
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN dep ensure
RUN go install
WORKDIR /go/bin
ENTRYPOINT ["./go-kardia"]
