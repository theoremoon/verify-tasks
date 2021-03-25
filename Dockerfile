FROM golang:1.16 AS builder

WORKDIR /go/src/app
ADD . /go/src/app
RUN go build -o /go/bin/app && go build -o /go/bin/app ./cmd/...

FROM debian:10-slim
COPY --from=builder  /go/bin/app /
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT /entrypoint.sh
