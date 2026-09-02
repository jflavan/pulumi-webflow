PROJECT_NAME := Pulumi Provider Webflow

PACK             := webflow
PACKDIR          := sdk
PROJECT          := github.com/JDetmar/pulumi-webflow
NODE_MODULE_NAME := @jdetmar/pulumi-webflow
NUGET_PKG_NAME   := Community.Pulumi.Webflow

PROVIDER        := pulumi-resource-${PACK}
PROVIDER_PATH   := provider
# Go symbol (relative to PROJECT) that receives the build version via -ldflags -X.
# The variable is `provider.Version` (provider/provider.go); .goreleaser.yml and
# .ci-mgmt.yaml reference the same fully-qualified path.
VERSION_PATH    := ${PROVIDER_PATH}.Version

PULUMI          := pulumi

SCHEMA_FILE     := provider/cmd/pulumi-resource-webflow/schema.json
export GOPATH   := $(shell go env GOPATH)

WORKING_DIR     := $(shell pwd)
TESTPARALLELISM := 4

# Override during CI using `make [TARGET] PROVIDER_VERSION=""` or by setting a PROVIDER_VERSION environment variable
# Local & branch builds will just used this fixed default version unless specified
PROVIDER_VERSION ?= 1.0.0-alpha.0+dev
# Use this normalised version everywhere rather than the raw input to ensure consistency.
# Requires pulumictl (installed by `mise install`, see .config/mise.toml). Can be overridden
# on the command line, e.g. `make provider VERSION_GENERIC=0.0.0-test`.
VERSION_GENERIC = $(shell pulumictl convert-version --language generic --version "$(PROVIDER_VERSION)")

# Fail loudly when the version could not be computed (pulumictl missing or PROVIDER_VERSION
# invalid) instead of silently baking an empty version into artifacts. Every target that
# embeds VERSION_GENERIC starts its recipe with $(require_version).
define require_version
@if [ -z "$(VERSION_GENERIC)" ]; then \
	echo "error: VERSION_GENERIC is empty: could not convert PROVIDER_VERSION='$(PROVIDER_VERSION)' with pulumictl." >&2; \
	echo "       Install pulumictl (run 'mise install'; see .config/mise.toml) or pass VERSION_GENERIC=<semver> explicitly." >&2; \
	exit 1; \
fi
endef

# Need to pick up locally pinned pulumi-langage-* plugins.
export PULUMI_IGNORE_AMBIENT_PLUGINS = true

.PHONY: ensure
ensure::
	go mod tidy

$(SCHEMA_FILE): provider
	$(PULUMI) package get-schema $(WORKING_DIR)/bin/${PROVIDER} | \
		jq 'del(.version) | (.language.go.importBasePath="github.com/JDetmar/pulumi-webflow/sdk/go/webflow")' > $(SCHEMA_FILE)

# Codegen generates the schema file and *generates* all sdks. This is a local process and
# does not require the ability to build all SDKs.
#
# To build the SDKs, use `make build_sdks`
#
# Required by CI (weekly-pulumi-update)
.PHONY: codegen
codegen: $(SCHEMA_FILE) sdk/dotnet sdk/go sdk/nodejs sdk/python sdk/java

.PHONY: sdk/dotnet sdk/go sdk/nodejs sdk/python sdk/java

sdk/nodejs: $(SCHEMA_FILE)
	$(require_version)
	rm -rf $@
	$(PULUMI) package gen-sdk --language nodejs $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	cp README.md ${PACKDIR}/nodejs/

sdk/java: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language java $(SCHEMA_FILE)
	# Generated settings.gradle references a non-existent 'lib' module; drop it for a single-module build.
	@if [ "$$(uname)" = "Darwin" ]; then \
		sed -i '' '/^include("lib")/d' sdk/java/settings.gradle; \
	else \
		sed -i '/^include("lib")/d' sdk/java/settings.gradle; \
	fi
	# Post-process build.gradle for Maven Central publishing
	# pulumi-java-gen doesn't support all POM metadata fields, so we use a Python script for reliable patching
	@echo "Post-processing Java SDK build.gradle for Maven Central..."
	@python3 scripts/patch-java-build-gradle.py sdk/java/build.gradle

sdk/python: $(SCHEMA_FILE)
	$(require_version)
	rm -rf $@
	$(PULUMI) package gen-sdk --language python $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	# Pulumi SDK generator doesn't set version in setup.py, so we patch it manually
	sed -i.bak 's/VERSION = "0.0.0"/VERSION = "${VERSION_GENERIC}"/' ${PACKDIR}/python/setup.py && rm ${PACKDIR}/python/setup.py.bak
	cp README.md ${PACKDIR}/python/

sdk/dotnet: $(SCHEMA_FILE)
	$(require_version)
	rm -rf $@
	$(PULUMI) package gen-sdk --language dotnet $(SCHEMA_FILE) --version "${VERSION_GENERIC}"

sdk/go: ${SCHEMA_FILE}
	$(require_version)
	rm -rf $@
	$(PULUMI) package gen-sdk --language go ${SCHEMA_FILE} --version "${VERSION_GENERIC}"
	GO_PKG_DIR=${PACKDIR}/go/webflow; \
	mkdir -p $$GO_PKG_DIR; \
	cp go.mod $$GO_PKG_DIR/go.mod; \
	cd $$GO_PKG_DIR && \
		go mod edit -module=github.com/JDetmar/pulumi-webflow/sdk/go/webflow && \
		go mod tidy

.PHONY: provider
provider: bin/${PROVIDER} bin/pulumi-gen-${PACK} # Required by CI

# Provider source files to track for rebuilds
PROVIDER_SRC := $(shell find provider -name '*.go')

bin/${PROVIDER}: $(PROVIDER_SRC)
	$(require_version)
	cd provider && go build -o $(WORKING_DIR)/bin/${PROVIDER} -ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION_GENERIC}" $(PROJECT)/${PROVIDER_PATH}/cmd/$(PROVIDER)

.PHONY: provider_debug
provider_debug:
	$(require_version)
	(cd provider && go build -o $(WORKING_DIR)/bin/${PROVIDER} -gcflags="all=-N -l" -ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION_GENERIC}" $(PROJECT)/${PROVIDER_PATH}/cmd/$(PROVIDER))

GO_TEST := go test -race -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM}

# Unit tests for the provider package (mocked HTTP, no API token needed).
.PHONY: test_provider
test_provider:
	cd provider && $(GO_TEST) -short -coverprofile="coverage.txt" ./...

# Integration tests in tests/ that drive the provider through the pulumi-go-provider
# integration server (no network access needed).
.PHONY: test_integration
test_integration:
	$(GO_TEST) ./tests/...

.PHONY: dotnet_sdk
dotnet_sdk: sdk/dotnet
	$(require_version)
	cd ${PACKDIR}/dotnet/&& \
		echo "${VERSION_GENERIC}" > version.txt && \
		dotnet build

.PHONY: go_sdk
go_sdk:	sdk/go

# The Node.js package is published from ${PACKDIR}/nodejs/bin (where tsc emits the
# JavaScript and .d.ts files), so package.json, README, LICENSE and .npmignore are
# copied there. bin/package.json is patched to point at the compiled entry points.
.PHONY: nodejs_sdk
nodejs_sdk: sdk/nodejs
	cd ${PACKDIR}/nodejs/ && \
		yarn install && \
		yarn run tsc
	cp README.md LICENSE ${PACKDIR}/nodejs/package.json ${PACKDIR}/nodejs/yarn.lock ${PACKDIR}/nodejs/.npmignore ${PACKDIR}/nodejs/bin/
	cd ${PACKDIR}/nodejs/bin && \
		jq '. + {main: (.main // "index.js"), types: (.types // "index.d.ts")}' package.json > package.json.tmp && \
		mv package.json.tmp package.json

.PHONY: python_sdk
python_sdk: sdk/python
	cp README.md ${PACKDIR}/python/
	cd ${PACKDIR}/python/ && \
		rm -rf ./bin/ ../python.bin/ && cp -R . ../python.bin && mv ../python.bin ./bin && \
		python3 -m venv venv && \
		./venv/bin/python -m pip install build && \
		cd ./bin && \
		../venv/bin/python -m build .

.PHONY: java_sdk
java_sdk:: PACKAGE_VERSION := $(VERSION_GENERIC)
java_sdk:: sdk/java
	$(require_version)
	cd sdk/java/ && \
		gradle --console=plain build

.PHONY: build
build:: provider build_sdks

.PHONY: build_sdks
build_sdks: dotnet_sdk go_sdk nodejs_sdk python_sdk java_sdk

# Required for the codegen action that runs in pulumi/pulumi
.PHONY: only_build
only_build:: build

.PHONY: lint
lint:
	golangci-lint --path-prefix provider --config .golangci.yml run --fix

# Type-check every TypeScript example against the locally built Node.js SDK (requires build_nodejs).
.PHONY: check_examples
check_examples:
	./scripts/check-examples.sh

.PHONY: install
install:: install_nodejs_sdk install_dotnet_sdk
	cp $(WORKING_DIR)/bin/${PROVIDER} ${GOPATH}/bin

# Run every Go test in the repository (provider unit tests + tests/ integration tests) from the module root.
.PHONY: test_all
test_all::
	$(GO_TEST) ./provider/... ./tests/...

.PHONY: install_dotnet_sdk
install_dotnet_sdk::
	rm -rf $(WORKING_DIR)/nuget/$(NUGET_PKG_NAME).*.nupkg
	mkdir -p $(WORKING_DIR)/nuget
	find . -name '*.nupkg' -print -exec cp -p {} ${WORKING_DIR}/nuget \;

.PHONY: install_python_sdk
install_python_sdk::
	#target intentionally blank

.PHONY: install_go_sdk
install_go_sdk::
	#target intentionally blank

.PHONY: install_java_sdk
install_java_sdk::
	#target intentionally blank

.PHONY: install_nodejs_sdk
install_nodejs_sdk::
	-yarn unlink --cwd $(WORKING_DIR)/sdk/nodejs/bin
	yarn link --cwd $(WORKING_DIR)/sdk/nodejs/bin

.PHONY: test
test:: test_provider test_integration

# Set these variables to enable signing of the windows binary
AZURE_SIGNING_CLIENT_ID ?=
AZURE_SIGNING_CLIENT_SECRET ?=
AZURE_SIGNING_TENANT_ID ?=
AZURE_SIGNING_KEY_VAULT_URI ?=
SKIP_SIGNING ?=

bin/jsign-6.0.jar:
	mkdir -p bin
	wget https://github.com/ebourg/jsign/releases/download/6.0/jsign-6.0.jar --output-document=bin/jsign-6.0.jar

sign-goreleaser-exe-amd64: GORELEASER_ARCH := amd64_v1
sign-goreleaser-exe-arm64: GORELEASER_ARCH := arm64

# Set the shell to bash to allow for the use of bash syntax.
sign-goreleaser-exe-%: SHELL:=/bin/bash
sign-goreleaser-exe-%: bin/jsign-6.0.jar
	@# Only sign windows binary if fully configured.
	@# Test variables set by joining with | between and looking for || showing at least one variable is empty.
	@# Move the binary to a temporary location and sign it there to avoid the target being up-to-date if signing fails.
	@set -e; \
	if [[ "${SKIP_SIGNING}" != "true" ]]; then \
		if [[ "|${AZURE_SIGNING_CLIENT_ID}|${AZURE_SIGNING_CLIENT_SECRET}|${AZURE_SIGNING_TENANT_ID}|${AZURE_SIGNING_KEY_VAULT_URI}|" == *"||"* ]]; then \
			echo "Can't sign windows binaries as required configuration not set: AZURE_SIGNING_CLIENT_ID, AZURE_SIGNING_CLIENT_SECRET, AZURE_SIGNING_TENANT_ID, AZURE_SIGNING_KEY_VAULT_URI"; \
			echo "To rebuild with signing delete the unsigned windows exe file and rebuild with the fixed configuration"; \
			if [[ "${CI}" == "true" ]]; then exit 1; fi; \
		else \
			file=dist/build-provider-sign-windows_windows_${GORELEASER_ARCH}/pulumi-resource-webflow.exe; \
			mv $${file} $${file}.unsigned; \
			az login --service-principal \
				--username "${AZURE_SIGNING_CLIENT_ID}" \
				--password "${AZURE_SIGNING_CLIENT_SECRET}" \
				--tenant "${AZURE_SIGNING_TENANT_ID}" \
				--output none; \
			ACCESS_TOKEN=$$(az account get-access-token --resource "https://vault.azure.net" | jq -r .accessToken); \
			java -jar bin/jsign-6.0.jar \
				--storetype AZUREKEYVAULT \
				--keystore "PulumiCodeSigning" \
				--url "${AZURE_SIGNING_KEY_VAULT_URI}" \
				--storepass "$${ACCESS_TOKEN}" \
				$${file}.unsigned; \
			mv $${file}.unsigned $${file}; \
			az logout; \
		fi; \
	fi

.PHONY: local_generate
local_generate: # Required by CI

.PHONY: generate_schema
generate_schema: ${SCHEMA_FILE} # Required by CI

.PHONY: generate_go build_go
generate_go: sdk/go # Required by CI
build_go: # Required by CI

.PHONY: generate_java build_java
generate_java: sdk/java # Required by CI
build_java: java_sdk # Required by CI

.PHONY: generate_python build_python
generate_python: sdk/python # Required by CI
build_python: python_sdk # Required by CI

.PHONY: generate_nodejs build_nodejs
generate_nodejs: sdk/nodejs # Required by CI
build_nodejs: nodejs_sdk # Required by CI

.PHONY: generate_dotnet build_dotnet
generate_dotnet: sdk/dotnet # Required by CI
build_dotnet: dotnet_sdk # Required by CI

bin/pulumi-gen-${PACK}: # Required by CI
	mkdir -p bin
	touch bin/pulumi-gen-${PACK}
