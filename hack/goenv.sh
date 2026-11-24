#!/bin/sh
# Local Go build environment for running inside the sandbox where the host
# GOPATH/module cache is not writable. Not part of the deliverable.
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export GOPATH="$ROOT/.cache/gopath"
export GOMODCACHE="$ROOT/.cache/gopath/pkg/mod"
export GOCACHE="$ROOT/.cache/gobuild"
export GOFLAGS=-mod=mod
export GOSUMDB=off
export GOPRIVATE='*'
export GOPROXY=https://proxy.golang.org,direct
exec go "$@"
