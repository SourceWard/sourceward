#!/bin/sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

version=0.1.2-test
"$repo_root/scripts/build-release.sh" "v$version" "$work_dir/release"
test "$(find "$work_dir/release" -type f -name 'sourceward_*' | wc -l | tr -d ' ')" -eq 6
test "$(wc -l <"$work_dir/release/checksums.txt" | tr -d ' ')" -eq 6

case "$(uname -s)" in
	Linux*) native_os=linux ;;
	Darwin*) native_os=darwin ;;
	*) native_os= ;;
esac
case "$(uname -m)" in
	x86_64|amd64) native_arch=amd64 ;;
	arm64|aarch64) native_arch=arm64 ;;
	*) native_arch= ;;
esac
if [ -n "$native_os" ] && [ -n "$native_arch" ]; then
	test "$("$work_dir/release/sourceward_${version}_${native_os}_${native_arch}" version)" = "$version"
fi

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
printf '%s\n' "sourceward_0.1.2_darwin_arm64: OK"
EOF
chmod +x "$tools/uname" "$tools/curl" "$tools/sha256sum"
cat >"$fixtures/sourceward_0.1.2_darwin_arm64" <<'EOF'
#!/bin/sh
exit 0
EOF
printf '%s\n' "fixture  sourceward_0.1.2_darwin_arm64" >"$fixtures/checksums.txt"

SOURCEWARD_DOWNLOAD_FIXTURES="$fixtures" PATH="$tools:$PATH" \
	"$repo_root/scripts/install-release.sh" v0.1.2 "$work_dir/install" \
	>"$work_dir/install.out" 2>"$work_dir/install.err"
test "$(cat "$work_dir/install.out")" = "$work_dir/install/sourceward_0.1.2_darwin_arm64"
grep -q "sourceward_0.1.2_darwin_arm64: OK" "$work_dir/install.err"
test -x "$work_dir/install/sourceward_0.1.2_darwin_arm64"

if grep -REn 'uses: [^ ]+@v[0-9]' "$repo_root/.github/workflows" "$repo_root/action.yml"
then
	echo "workflow action is not pinned to an immutable revision" >&2
	exit 1
fi
