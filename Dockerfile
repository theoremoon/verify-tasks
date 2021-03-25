FROM golang:1.16 AS builder

WORKDIR /go/src/app
ADD . /go/src/app
RUN go build -o /go/bin/app/ && go build -o /go/bin/app/ ./cmd/...

RUN chmod +x /entrypoint.sh

ENTRYPOINT /entrypoint.sh

FROM debian:10-slim
COPY --from=builder /go/bin/app/verify-tasks /verify-tasks
COPY --from=builder /go/bin/app/result-md /result-md
COPY entrypoint.sh /entrypoint.sh
