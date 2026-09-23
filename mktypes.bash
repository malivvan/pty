#!/usr/bin/env bash
#
# Regenerates the ztypes_*.go files, which describe the C types and constants
# the BSD pseudo-terminal interfaces hand to this package.
#
# The files are produced by cgo -godefs, which needs a C compiler and the
# headers of the target system, so run this script on the target itself (or in
# a container of it):
#
#	GOOS=freebsd GOARCH=amd64 ./mktypes.bash
#	GOOS=freebsd GOARCH=arm64 ./mktypes.bash
#	GOOS=netbsd GOARCH=amd64 ./mktypes.bash
#	GOOS=openbsd GOARCH=amd64 ./mktypes.bash
#	GOOS=dragonfly GOARCH=amd64 ./mktypes.bash
#
# The C integer types this script used to generate as well (C.int, C.uint) are
# gone: the Linux implementation declares the int32/uint32 values its ioctls
# need directly, which is what those types ever stood for.

set -euo pipefail

GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"
export GOOS GOARCH

case "$GOOS" in
freebsd | dragonfly)
	# The structure layouts and the maximum device name length differ between
	# the architectures of these systems, hence one file per architecture.
	out="ztypes_${GOOS}_${GOARCH}.go"
	tags="${GOARCH} && ${GOOS}"
	;;
netbsd | openbsd)
	# A single file per system: the layouts are the same on every
	# architecture and the file name already implies the build constraint.
	out="ztypes_${GOOS}.go"
	tags="${GOOS}"
	;;
*)
	echo "mktypes: no generated types are defined for ${GOOS}/${GOARCH}" >&2
	exit 1
	;;
esac

echo "mktypes: generating ${out}" >&2

# The build constraint is written out explicitly so that the generated file
# states which systems it belongs to, even where the file name already implies
# it.
{
	printf '//go:build %s\n\n' "$tags"
	go tool cgo -godefs "types_${GOOS}.go"
} | gofmt >"$out"

echo "mktypes: wrote ${out}" >&2
