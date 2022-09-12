FROM golang:1.19 AS builder
COPY . /tmp/src/
RUN cd /tmp/src/cmd && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags '-s -w -extldflags "-static"' -o /tmp/bin/promwrap

FROM scratch
COPY --from=builder /tmp/bin/promwrap /usr/bin/promwrap
ENTRYPOINT [] # This image is not intended to be run, the binary is distributed as a "mixin" for other images