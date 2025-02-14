# SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
#
# SPDX-License-Identifier: MIT

.PHONY: all build tests clean

all: build

build:
	go build -o params2env

tests:
	go test -v -cover ./...

clean:
	rm -f params2env
	rm -f coverage.out
