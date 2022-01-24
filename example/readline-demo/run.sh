#!/usr/bin/env bash
set -eou pipefail

setup() {
	go mod tidy
	go get
}

tidy() {
	goimports -w *.go
	gofmt -w *.go
}

run() {
	go run .
}

cmd() {
	local CMD="$@"
	ansi >&2 --yellow --italic "> $CMD"
	run <<EOF
$CMD
EOF
}

tidy || { setup && tidy; }

rm plugins/*.so 2>/dev/null || true
cmd gosh list -f
cmd gosh build commands
cmd gosh build all
cmd gosh list -p
cmd gosh load
cmd help

exit

cmd goodbye
#cmd sleep
cmd sys help

exit

if ! run; then
	setup
	run
fi
