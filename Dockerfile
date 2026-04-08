FROM golang:1.26-trixie AS builder

COPY cmd cmd
COPY pkg pkg
COPY go.mod go.mod
COPY go.sum go.sum

RUN go build -o build/mxl-exporter ./cmd/mxl-exporter

FROM debian:trixie-slim
COPY --from=builder /go/build/mxl-exporter /usr/bin/mxl-exporter
ENTRYPOINT ["/usr/bin/mxl-exporter"]
