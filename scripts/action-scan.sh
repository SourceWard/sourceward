#!/usr/bin/env bash

set -u

binary=$1
root=$2
format=$3
fail_on=$4
check_lock=$5
include_personal=$6
sarif_file=$7

args=(scan --root "$root" --format "$format" --fail-on "$fail_on")
if [[ "$check_lock" == "true" ]]; then
	args+=(--check-lock)
fi
if [[ "$include_personal" == "true" ]]; then
	args+=(--include-personal)
fi

if [[ "$format" == "sarif" ]]; then
	"$binary" "${args[@]}" >"$sarif_file"
else
	"$binary" "${args[@]}"
fi
