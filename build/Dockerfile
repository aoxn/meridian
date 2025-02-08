##############      builder       #############
#FROM golang:1.11.5 AS builder
#
#WORKDIR /go
#COPY . .
#RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install ./...
#

############# cloud controller manager #############
FROM alpine:3.8

RUN apk add --no-cache --update ca-certificates
COPY bin/etcdctl /usr/bin/
COPY bin/wdrip.amd64 /wdrip
COPY bin/etcdm.sh /usr/bin/

ENTRYPOINT  ["/wdrip"]
