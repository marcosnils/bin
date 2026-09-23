.PHONY: help build verify download coverage android-ndk release

NO_COLOR=\033[0m
GREEN=\033[32;01m
YELLOW=\033[33;01m
RED=\033[31;01m

NDK_VERSION ?= r30
NDK_SHA1 ?= 5107f898313790e449e87eee2183d9a20602dee9
NDK_DIR ?= $(HOME)/.cache/android-ndk
ANDROID_NDK_HOME ?= $(NDK_DIR)/android-ndk-$(NDK_VERSION)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[33m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	go build .

clean: ## Clean artefacts
	rm -rf bin

lint: # Run lint
	go fmt ./...
	go vet ./...

test: ## Run all tests
	go test ./...

download: ## Download dependencies
	go mod download
	go mod tidy

verify: download ## Code verification
	gofmt -w -s ./.
	golangci-lint run

android-ndk: ## Install the Android NDK (linux only) used for android builds
	@if [ ! -d "$(ANDROID_NDK_HOME)" ]; then \
		mkdir -p "$(NDK_DIR)" && \
		curl -fL -o "$(NDK_DIR)/ndk.zip" "https://dl.google.com/android/repository/android-ndk-$(NDK_VERSION)-linux.zip" && \
		echo "$(NDK_SHA1)  $(NDK_DIR)/ndk.zip" | sha1sum -c - && \
		unzip -q "$(NDK_DIR)/ndk.zip" -d "$(NDK_DIR)" && \
		rm "$(NDK_DIR)/ndk.zip"; \
	fi

release: android-ndk ## Release with goreleaser (installs the Android NDK)
	ANDROID_NDK_HOME="$(ANDROID_NDK_HOME)" goreleaser release --clean
