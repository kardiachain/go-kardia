FROM golang:1.19.4-alpine as builder
RUN apk update && apk add build-base cmake gcc git
WORKDIR /go/src/github.com/kardiachain/go-kardia
ADD . .
WORKDIR /go/src/github.com/kardiachain/go-kardia/cmd/kaigo
RUN go install
WORKDIR /go/bin

FROM alpine:3.18
RUN mkdir -p /go/bin/cfg
COPY cmd/cfg/* /go/bin/cfg/
ENV PATH="${PATH}:/go/bin"
WORKDIR /go/bin
COPY --from=builder /go/bin/* .
ENTRYPOINT ["./kaigo"]
