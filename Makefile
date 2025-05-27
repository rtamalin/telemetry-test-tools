toplevel_dir := $(dir $(abspath $(firstword $(MAKEFILE_LIST))))

go_no_proxy ?= github.com/SUSE

export go_coverage_profile = $(abspath $(toplevel_dir)/.coverage.out)
export GONOPROXY=$(go_no_proxy)

.DEFAULT_GOAL := build

ifeq ($(MAKELEVEL),0)

SUBDIRS = \
  cmd/batcher

TOPDIR_TARGETS = mod-tidy mod-download mod-update test-mod-update test-coverage
SUBDIR_TARGETS = fmt vet build build-only clean test test-clean test-verbose

.PHONY: $(TOPDIR_TARGETS) $(SUBDIR_TARGETS)

$(SUBDIR_TARGETS)::
	$(foreach subdir, $(SUBDIRS), $(MAKE) -C $(subdir) $@ || exit 1;)

mod-tidy:
	go mod tidy -x

mod-download:
	go mod download -x

mod-update:
	go get -u -x && \
	go mod tidy

test-mod-update:
	go get -u -t -x && \
	go mod tidy

test-coverage: test
	go tool cover --func=$(go_coverage_profile)



else
include Makefile.golang
endif
