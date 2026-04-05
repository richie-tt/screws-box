.PHONY: build sbom

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o screws-box ./cmd/screwsbox

# Generate Software Bill of Materials (CycloneDX format).
# Install: go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest
sbom:
	cyclonedx-gomod mod -json -output sbom.json
