FROM golang:1.16 AS builder

WORKDIR /go/src/app
ADD . /go/src/app
RUN go build -o /go/bin/app && go build -o /go/bin/app ./cmd/...

FROM gcr.io/distroless/base-debian10
COPY --from=builder  /go/bin/app /
COPY entrypoint.sh /entrypoint.sh

CMD /verify-tasks -dir 
