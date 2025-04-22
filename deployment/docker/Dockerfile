FROM golang:1.21.6-alpine3.19 as builder

RUN apk add git

WORKDIR /go/src/github.com/eclipse-xfsc/trusted-info-hub

ARG APP_REPO_TAG

ADD . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X main.Version=$APP_REPO_TAG" -mod=vendor -o /tmp/infohub ./cmd/infohub/...

FROM alpine:3.19 as runner

COPY --from=builder /tmp/infohub /opt/infohub

WORKDIR /opt

CMD ["./infohub"]
