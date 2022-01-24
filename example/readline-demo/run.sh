#!/usr/bin/env bash
set -eou pipefail
#set -x
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
  >&2  ansi --yellow --italic "> $CMD"
	run <<EOF
$CMD
EOF
}

tidy || { setup && tidy; }

#cmd help
#cmd gosh build commands
rm plugins/*.so 2>/dev/null||true
#cmd gosh list -f
cmd gosh build all
cmd gosh list -p
cmd gosh load

exit

cmd hello
cmd goodbye
#cmd sleep
cmd sys help

exit

if ! run; then
	setup
	run
fi
