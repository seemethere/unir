FROM golang:1.9.2-alpine as build

RUN apk --update add make

COPY . /go/src/github.com/seemethere/unir
WORKDIR /go/src/github.com/seemethere/unir

RUN make clean build

FROM alpine:latest
RUN apk --update add ca-certificates
COPY --from=build /go/src/github.com/seemethere/unir/build/unir /unir
ENTRYPOINT ["/unir"]
