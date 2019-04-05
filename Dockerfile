FROM golang:1.10
RUN mkdir -p "$GOPATH/src/github.com/kardiachain/go-kardia"
WORKDIR /go/src/github.com/kardiachain/go-kardia
RUN echo "deb http://download.opensuse.org/repositories/network:/messaging:/zeromq:/release-stable/Debian_9.0/ ./" >> /etc/apt/sources.list
RUN wget https://download.opensuse.org/repositories/network:/messaging:/zeromq:/release-stable/Debian_9.0/Release.key -O- | apt-key add
RUN apt-get update && apt-get install -y libzmq3-dev
ADD . .
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN dep ensure
RUN go install
WORKDIR /go/bin
ENTRYPOINT ["./go-kardia"]
