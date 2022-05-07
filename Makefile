ifeq ($(GOOS),windows)
GOLANGCI_LINT:=golangci-lint.exe
else
GOLANGCI_LINT:=golangci-lint
endif

TEST_OUTPUT_FILE_PREFIX ?= ./test_report

PROG=bin/ServiceCar

MODE_PIE=-buildmode=pie

SRCS=./cmd/ServiceCar

INSTALL_PREFIX=/usr/local/ServiceCar

CONF_INSTALL_PREFIX=/usr/local/ServiceCar

# git commit hash
COMMIT_HASH=$(shell git rev-parse --short HEAD || echo "GitNotFound")

BUILD_DATE=$(shell date '+%Y-%m-%d %H:%M:%S')

CFLAGS = -ldflags "-s -w  -X \"main.BuildTag=$(CI_COMMIT_REF_NAME)\" -X \"main.BuildVersion=${COMMIT_HASH}\" -X \"main.BuildDate=$(BUILD_DATE)\""

all: build

build:
	if [ ! -d "./bin/" ]; then \
	mkdir bin; \
	fi
	go build $(CFLAGS)  -o $(PROG) $(SRCS)

race:
	if [ ! -d "./bin/" ]; then \
    	mkdir bin; \
    	fi
	go build $(CFLAGS) -race -o $(PROG) $(SRCS)

run:
	go run cmd/ServiceCar/main.go -c bin/sc.yaml

run_dev:
	export MSP_TARGET_ADDRESS=127.0.0.1:8002
	go run  --race   cmd/ServiceCar/main.go -c config/sc_config_dev.yaml

run_remote:
	dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./bin/ServiceCar -- -c config/sc3_config_dev.yaml
# pie
pie:
	if [ ! -d "./bin/" ]; then \
	mkdir bin; \
	fi
	go build $(CFLAGS) $(MODE_PIE)  -o $(PROG) $(SRCS)

upx:
	mv bin/ServiceCar bin/ServiceCarori
	upx -6 bin/ServiceCarori -o bin/ServiceCar
	rm bin/ServiceCarori

.PHONY: test-deps
test-deps:
	# The desire here is to download this test dependency without polluting go.mod
	# In golang >=1.16 there is a new way to do this with `go install gotest.tools/gotestsum@latest`
	# But this doesn't work with <=1.15.
	# (see: https://golang.org/ref/mod#go-install)
	command -v gotestsum || go install gotest.tools/gotestsum@latest

################################################################################
# Target: test                                                                 #
################################################################################
.PHONY: test
test: test-deps
	gotestsum --jsonfile $(TEST_OUTPUT_FILE_PREFIX)_unit.json --format standard-quiet -- ./pkg/... ./utils/... ./cmd/... $(COVERAGE_OPTS) --tags=unit
	go test ./tests/...


################################################################################
# Target: lint                                                                 #
################################################################################
# Due to https://github.com/golangci/golangci-lint/issues/580, we need to add --fix for windows
.PHONY: lint
lint:
	$(GOLANGCI_LINT) run --timeout=20m