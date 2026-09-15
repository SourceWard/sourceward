#!/bin/sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

version=v0.1.0-test
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
do
	os=${target%/*}
	arch=${target#*/}
	suffix=
	if [ "$os" = windows ]; then suffix=.exe; fi
	output="$work_dir/sourceward_${version#v}_${os}_${arch}${suffix}"
	GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build \
		-trimpath \
		-ldflags="-X github.com/SourceWard/sourceward/internal/cli.Version=$version" \
		-o "$output" "$repo_root/cmd/sourceward"
	test -s "$output"
done

capture="$work_dir/arguments"
fake="$work_dir/sourceward"
cat >"$fake" <<'EOF'
#!/bin/sh
printf '%s\n' "$@" >"$SOURCEWARD_ARGUMENT_CAPTURE"
exit 6
EOF
chmod +x "$fake"

set +e
SOURCEWARD_ARGUMENT_CAPTURE="$capture" "$repo_root/scripts/action-scan.sh" \
	"$fake" repository json medium true true result.sarif
status=$?
set -e
test "$status" -eq 6
cat >"$work_dir/expected" <<'EOF'
scan
--root
repository
--format
json
--fail-on
medium
--check-lock
--include-personal
EOF
cmp "$work_dir/expected" "$capture"

if "$repo_root/scripts/install-release.sh" invalid "$work_dir/install" >"$work_dir/install.out" 2>"$work_dir/install.err"
then
	echo "invalid release version unexpectedly succeeded" >&2
	exit 1
fi

tools="$work_dir/tools"
fixtures="$work_dir/fixtures"
mkdir -p "$tools" "$fixtures"
cat >"$tools/uname" <<'EOF'
#!/bin/sh
case "$1" in
	-s) printf '%s\n' Darwin ;;
	-m) printf '%s\n' arm64 ;;
	*) exit 1 ;;
esac
EOF
cat >"$tools/curl" <<'EOF'
#!/bin/sh
while [ "$#" -gt 0 ]; do
	if [ "$1" = "-o" ]; then
		output=$2
		shift 2
		continue
	fi
	url=$1
	shift
done
cp "$SOURCEWARD_DOWNLOAD_FIXTURES/${url##*/}" "$output"
EOF
cat >"$tools/sha256sum" <<'EOF'
#!/bin/sh
cat >/dev/null
printf '%s\n' "sourceward_0.1.1_darwin_arm64: OK"
EOF
chmod +x "$tools/uname" "$tools/curl" "$tools/sha256sum"
cat >"$fixtures/sourceward_0.1.1_darwin_arm64" <<'EOF'
#!/bin/sh
exit 0
EOF
printf '%s\n' "fixture  sourceward_0.1.1_darwin_arm64" >"$fixtures/checksums.txt"

SOURCEWARD_DOWNLOAD_FIXTURES="$fixtures" PATH="$tools:$PATH" \
	"$repo_root/scripts/install-release.sh" v0.1.1 "$work_dir/install" \
	>"$work_dir/install.out" 2>"$work_dir/install.err"
test "$(cat "$work_dir/install.out")" = "$work_dir/install/sourceward_0.1.1_darwin_arm64"
grep -q "sourceward_0.1.1_darwin_arm64: OK" "$work_dir/install.err"
test -x "$work_dir/install/sourceward_0.1.1_darwin_arm64"

if grep -REn 'uses: [^ ]+@v[0-9]' "$repo_root/.github/workflows" "$repo_root/action.yml"
then
	echo "workflow action is not pinned to an immutable revision" >&2
	exit 1
fi
