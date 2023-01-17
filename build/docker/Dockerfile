FROM golang:1.17.13-alpine3.16 AS builder
WORKDIR /
COPY . .

RUN apk add --update git && apk upgrade --available
RUN go build -o /gravity-adapter-nats ./cmd/gravity-adapter-nats/gravity-adapter-nats.go

FROM alpine:3.17.1
RUN apk update && apk upgrade --available
COPY --from=builder /gravity-adapter-nats /gravity-adapter-nats
COPY ./configs /configs
COPY ./settings /settings
COPY ./build/docker/startup.sh /startup.sh

RUN chmod 700 -R /settings && chown -R 1001  /settings
USER 1001

CMD ["sh", "/startup.sh"]
