#!/bin/sh

set -eu

version=${1:?version is required}
destination=${2:?destination is required}
repository=${SOURCEWARD_REPOSITORY:-SourceWard/sourceward}

if ! printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([-.][A-Za-z0-9.]+)?$'; then
	echo "version must be a release tag such as v0.1.0" >&2
	exit 2
fi

case "$(uname -s)" in
	Linux*) os=linux ;;
	Darwin*) os=darwin ;;
	MINGW*|MSYS*|CYGWIN*) os=windows ;;
	*) echo "unsupported operating system" >&2; exit 3 ;;
esac

case "$(uname -m)" in
	x86_64|amd64) arch=amd64 ;;
	arm64|aarch64) arch=arm64 ;;
	*) echo "unsupported architecture" >&2; exit 3 ;;
esac

suffix=
if [ "$os" = windows ]; then
	suffix=.exe
fi
filename="sourceward_${version#v}_${os}_${arch}${suffix}"
base_url="https://github.com/${repository}/releases/download/${version}"

mkdir -p "$destination"
curl --proto '=https' --tlsv1.2 -fsSL "$base_url/$filename" -o "$destination/$filename"
curl --proto '=https' --tlsv1.2 -fsSL "$base_url/checksums.txt" -o "$destination/checksums.txt"

(
	cd "$destination"
	if command -v sha256sum >/dev/null 2>&1; then
		grep "  $filename\$" checksums.txt | sha256sum -c -
	else
		grep "  $filename\$" checksums.txt | shasum -a 256 -c -
	fi
)

chmod +x "$destination/$filename"
printf '%s\n' "$destination/$filename"
