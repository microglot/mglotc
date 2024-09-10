# © 2024 Microglot LLC
#
# SPDX-License-Identifier: Apache-2.0

.PHONY : update bin
.PHONY : lint test integration coverage
.PHONY : clean clean/coverage clean/bin

PROJECT_PATH = $(shell pwd -L)
GOFLAGS ::= ${GOFLAGS}
BUILD_DIR = $(PROJECT_PATH)/.build
TOOLS_DIR = $(PROJECT_PATH)/internal/tools
TOOLS_FILE = $(TOOLS_DIR)/tools.go
TOOLS_MOD = $(TOOLS_DIR)/go.mod
TOOLS_SUM = $(TOOLS_DIR)/go.sum
GOTOOLS = $(shell grep '_' $(TOOLS_DIR)/tools.go | sed 's/[[:space:]]*_//g' | sed 's/\"//g')
BIN_DIR = $(PROJECT_PATH)/.bin
COVER_DIR = $(BUILD_DIR)/.coverage
COVERAGE_UNIT = $(COVER_DIR)/unit.out
COVERAGE_UNIT_INTERCHANGE = $(COVERAGE_UNIT:.out=.interchange)
COVERATE_UNIT_HTML = $(COVERAGE_UNIT:.out=.html)
COVERAGE_UNIT_XML = $(COVERAGE_UNIT:.out=.xml)
COVERAGE_COMBINED = $(COVER_DIR)/combined.out
COVERAGE_COMBINED_INTERCHANGE = $(COVERAGE_COMBINED:.out=.interchange)
COVERAGE_COMBINED_HTML = $(COVERAGE_COMBINED:.out=.html)
COVERAGE_COMBINED_XML = $(COVERAGE_COMBINED:.out=.xml)
GOIMPORT_LOCAL = github.com/microglot
GOLANGCILINT_CONFIG = $(PROJECT_PATH)/.golangci.yaml
GOCMD = GOFLAGS=$(GOFLAGS) go
BUILD_MODE = local
BUILD_FLAGS = --clean
ifneq ($(BUILD_MODE),tag)
	BUILD_FLAGS = --clean --snapshot
endif



#######
# https://stackoverflow.com/a/10858332
check_defined = \
    $(strip $(foreach 1,$1, \
        $(call __check_defined,$1,$(strip $(value 2)))))
__check_defined = \
    $(if $(value $1),, \
      $(error Undefined $1$(if $2, ($2))))
#######

build: | $(BUILD_DIR) $(BIN_DIR)
	@ $(BIN_DIR)/goreleaser build $(BUILD_FLAGS)

release: | $(BUILD_DIR) $(BIN_DIR)
	@ $(BIN_DIR)/goreleaser release --clean

test: test/unit test/descriptor-diff

test/lint: | $(BIN_DIR)
	@ GOFLAGS="$(GOFLAGS)" \
	$(BIN_DIR)/golangci-lint run \
		--config $(GOLANGCILINT_CONFIG)

test/license:
	@ pipx run reuse lint

test/unit: $(COVERAGE_UNIT) | $(BIN_DIR)

test/descriptor-diff:
	@ PATH="${PATH}:$(BIN_DIR)" ./descriptor_diff.bash minimal
	@ PATH="${PATH}:$(BIN_DIR)" ./descriptor_diff.bash message
	@ PATH="${PATH}:$(BIN_DIR)" ./descriptor_diff.bash comment
	@ PATH="${PATH}:$(BIN_DIR)" ./descriptor_diff.bash map
	@ PATH="${PATH}:$(BIN_DIR)" ./descriptor_diff.bash descriptor

test/coverage: $(COVER_DIR) $(COVERAGE_UNIT) $(COVERAGE_UNIT_INTERCHANGE) $(COVERATE_UNIT_HTML) $(COVERAGE_UNIT_XML) $(COVERAGE_COMBINED) $(COVERAGE_COMBINED_INTERCHANGE) $(COVERAGE_COMBINED_HTML) $(COVERAGE_COMBINED_XML) | $(BIN_DIR)
	@ $(GOCMD) tool cover -func $(COVERAGE_COMBINED)

tools: $(TOOLS_FILE) $(TOOLS_MOD) $(TOOLS_SUM) | $(BIN_DIR)
	@ cd $(TOOLS_DIR) && GOBIN=$(BIN_DIR) $(GOCMD) install $(GOTOOLS)
tools/update:
	@ cd $(TOOLS_DIR) && GOBIN=$(BIN_DIR) $(GOCMD) get -u
	@ cd $(TOOLS_DIR) && GOBIN=$(BIN_DIR) $(GOCMD) mod tidy

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

generate: | $(BIN_DIR)
	# Run any code generation steps.
	GO111MODULE=on \
	GOFLAGS="$(GOFLAGS)" \
	PATH="${PATH}:$(BIN_DIR)" \
	go generate ./...
	$(MAKE) fmt

fmt: | $(BIN_DIR)
	@ GOFLAGS="$(GOFLAGS)" \
	$(BIN_DIR)/goimports -w -v \
		-local $(GOIMPORT_LOCAL) \
		$(shell find . -type f -name '*.go' -not -path "./vendor/*")

clean: clean/test clean/tools
	@:$(call check_defined,BUILD_DIR)
	@ rm -rf "$(BUILD_DIR)"
clean/test:
	@:$(call check_defined,COVER_DIR)
	@ rm -rf "$(COVER_DIR)"
clean/tools:
	@:$(call check_defined,BIN_DIR)
	@ rm -rf "$(BIN_DIR)"


$(COVERAGE_UNIT): $(shell find . -type f -name '*.go' -not -path "./vendor/*") | $(COVER_DIR)
	$(GOCMD) test \
		-v \
		-cover \
		-race \
		-coverprofile="$(COVERAGE_UNIT)" \
		./...

$(COVER_DIR)/%.interchange: $(COVER_DIR)/%.out
	@ GOFLAGS="$(GOFLAGS)" \
	$(BIN_DIR)/gocov convert $< > $@

$(COVER_DIR)/%.xml: $(COVER_DIR)/%.interchange
	@ cat $< | \
	GOFLAGS="$(GOFLAGS)" \
	$(BIN_DIR)/gocov-xml > $@

$(COVER_DIR)/%.html: $(COVER_DIR)/%.interchange
	@ cat $< | \
	GOFLAGS="$(GOFLAGS)" \
	$(BIN_DIR)/gocov-html > $@

$(COVERAGE_COMBINED):
	@ GOFLAGS="$(GOFLAGS)" \
 	$(BIN_DIR)/gocovmerge $(COVER_DIR)/*.out > $(COVERAGE_COMBINED)

$(COVER_DIR):
	@ mkdir -p $(COVER_DIR)

