#!/bin/sh

set -eu

tag=${1:?release tag is required}
destination=${2:-dist}

if ! printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([-.][A-Za-z0-9.]+)?$'; then
	echo "release tag must be a version such as v0.1.2" >&2
	exit 2
fi

version=${tag#v}
mkdir -p "$destination"

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
do
	os=${target%/*}
	arch=${target#*/}
	suffix=
	if [ "$os" = windows ]; then suffix=.exe; fi
	output="$destination/sourceward_${version}_${os}_${arch}${suffix}"
	GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build \
		-trimpath \
		-ldflags="-s -w -X github.com/SourceWard/sourceward/internal/cli.Version=$version" \
		-o "$output" ./cmd/sourceward
done

(
	cd "$destination"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum sourceward_* >checksums.txt
	else
		shasum -a 256 sourceward_* >checksums.txt
	fi
)
