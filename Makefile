.PHONY: all
all: mxl-exporter

.PHONY: clean
clean:
	rm -f build/mxl-exporter

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: mxl-exporter
mxl-exporter: tidy
	CGO_ENABLED=0 go build -o build/mxl-exporter ./cmd/mxl-exporter
