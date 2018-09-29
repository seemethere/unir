ARG GOLANG_VERSION=1.11.0
FROM golang:${GOLANG_VERSION}-alpine as build

RUN apk --update add make

COPY . /go/src/github.com/seemethere/unir
WORKDIR /go/src/github.com/seemethere/unir

RUN make clean build

FROM alpine:latest
RUN apk --update add ca-certificates
COPY --from=build /go/src/github.com/seemethere/unir/build/unir /unir
ENTRYPOINT ["/unir"]
