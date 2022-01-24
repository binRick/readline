#!/usr/bin/env bash
set -eou pipefail

setup(){
  go mod tidy
  go get
}
tidy(){
  goimports -w *.go
  gofmt -w *.go
}

run(){
  go run . 
}

tidy
run || { setup; run; }
