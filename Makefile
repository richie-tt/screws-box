.PHONY: build sbom

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o screws-box ./cmd/screwsbox

# Generate Software Bill of Materials (CycloneDX format).
# Install: go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest
sbom:
	cyclonedx-gomod mod -json -output sbom.json
