# Project variables
BINARY_NAME := iris-ether 
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get

# Directories
SRC_DIR := ./cmd/main/main.go
BUILD_DIR := ./build/bin
# BENCH_CACHE_DIR := ./.bench-cache
# BENCH_DATA_DIR := ./.bench-data
# BENCH_LOG_DIR := ./.bench-log
# BENCH_DIR := ~/Dataset/etherscan-dataset-from-cast/

# Build flags
LDFLAGS := -ldflags="-s -w"

# Debug flags
LDDEBUG := -gcflags="all=-N -l"

TARGET := ./dinoeggs.bin-runtime
TARGETNAME := dinoeggs

# Test Target
# TESTGROUP := TestLoopAnalysis
# SUBTEST := Multiple_Nested_Loops_with_Shared_Header 

# Benchmark
# BENCHMARK_EXAMPLE := version_parse
# BENCHMARK := construction 
# BENCHMARK_BUILD := build_clean
# BENCHMARK_BUILD_INSTRUMENT := build_instrument
# BATCH_SIZE := 100
# BATCH_SCRIPT := ./run_benchmarks.sh

# dot
# DOT := dot
# SVG := -Tsvg

# Default target
all: build

# Build the application
build:
	@rm -rf $(BUILD_DIR)
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -v -x $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(SRC_DIR)

debug:
	@rm -rf $(BUILD_DIR)
	@echo "Building $(BINARY_NAME) in debug mode..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDDEBUG) -o $(BUILD_DIR)/$(BINARY_NAME) $(SRC_DIR)

test:
	@echo "Testing $(TESTGROUP)/$(SUBTEST)"
	@cd ./cfg ;$(GOTEST) -v -run $(TESTGROUP)/$(SUBTEST)
	@cd ..

statis:
	@python3 ./scripts/statis_total_coverage_multi_file.py ./.bench-log/

git-commit-all:
	@changed_files=$$(git status --porcelain | wc -l); \
	if [ $$changed_files -gt 1 ]; then \
		echo "Multiple changed files detected:"; \
		git status --porcelain; \
		echo "Error: Only one file can be committed at a time"; \
		exit 1; \
	fi
	@git add .
	@$(MAKE) git-commit-staged	

git-commit-staged:
	@lazygit
	@git diff --staged > diff.log
	@aichat -f diff.log -- "Generate a concise and standard Git commit message. The message should clearly describe the change, using imperative mood (e.g., 'fix', 'add', 'update','feat','doc'). '." > commit.log
	@echo "+++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"
	@cat commit.log
	@echo "+++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"
	@$(MAKE) ask_commit

ask_commit:
	@echo "Do you want to commit the changes? (y/n/e/r)"
	@read -p "" confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		git commit -F commit.log; \
		git push; \
	elif [ "$$confirm" = "e" ] || [ "$$confirm" = "E" ]; then \
		nvim commit.log; \
		echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"; \
		cat commit.log; \
		echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"; \
		$(MAKE) ask_commit; \
	elif [ "$$confirm" = "r" ] || [ "$$confirm" = "R" ]; then \
		aichat -f diff.log -- "Generate a concise and standard Git commit message. The message should clearly describe the change, using imperative mood (e.g., 'fix', 'add', 'update','feat','doc'). '." > commit.log; \
		echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"; \
		cat commit.log; \
		echo "++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++"; \
		$(MAKE) ask_commit; \
	else \
		echo "Commit canceled."; \
	fi
.PHONY: build debug
