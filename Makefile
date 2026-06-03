PROJECT = hrbridge
VERSION = 0.1.0

GO ?= go
GOLANGCI_LINT_VERSION ?= v2.12.2
BUILD_DIR ?= build
PACKAGE_DIR ?= $(BUILD_DIR)/package
LDFLAGS = -s -w

.PHONY: all clean native darwin aarch64 mipsel mips
.PHONY: package package-aarch64 package-mipsel package-mips feed
.PHONY: generate fmt-check test lint ci shell-check smoke-local smoke-router smoke-router-write smoke-router-service smoke-router-rci

all: aarch64 mipsel mips
package: package-aarch64 package-mipsel package-mips

feed: package
	BUILD_DIR=$(abspath $(BUILD_DIR)) VERSION=$(VERSION) ./scripts/build_feed.sh

generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.0 --config api/oapi-codegen.types.yaml api/openapi.yaml

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './build/*'))" || { \
		echo "Go files must be formatted with gofmt:"; \
		gofmt -l $$(find . -name '*.go' -not -path './build/*'); \
		exit 1; \
	}

test:
	$(GO) test ./...

lint:
	$(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

ci: generate fmt-check test native shell-check

shell-check:
	@for script in packaging/S99hrbridge scripts/*.sh; do \
		sh -n "$$script"; \
	done

smoke-local: native
	./scripts/smoke_local.sh

smoke-router:
	MODE=readonly ./scripts/smoke_router.sh

smoke-router-write:
	MODE=write ./scripts/smoke_router.sh

smoke-router-service:
	MODE=service ./scripts/smoke_router.sh

smoke-router-rci:
	MODE=rci ./scripts/smoke_router.sh

native:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT) ./cmd/hrbridge

darwin:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-darwin-arm64 ./cmd/hrbridge

aarch64:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-aarch64 ./cmd/hrbridge

mipsel:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-mipsel ./cmd/hrbridge

mips:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT)-mips ./cmd/hrbridge

package-aarch64: aarch64
	$(MAKE) package-ipk ARCH=aarch64-3.10 BIN=$(BUILD_DIR)/$(PROJECT)-aarch64

package-mipsel: mipsel
	$(MAKE) package-ipk ARCH=mipsel-3.4 BIN=$(BUILD_DIR)/$(PROJECT)-mipsel

package-mips: mips
	$(MAKE) package-ipk ARCH=mips-3.4 BIN=$(BUILD_DIR)/$(PROJECT)-mips

package-ipk:
	@test -n "$(ARCH)"
	@test -n "$(BIN)"
	@rm -rf $(PACKAGE_DIR)/$(ARCH)
	@mkdir -p $(PACKAGE_DIR)/$(ARCH)/control
	@install -d -m 0700 $(PACKAGE_DIR)/$(ARCH)/data/opt/etc/hrbridge
	@mkdir -p $(PACKAGE_DIR)/$(ARCH)/data/opt/etc/init.d
	@install -m 0755 $(BIN) $(PACKAGE_DIR)/$(ARCH)/data/opt/etc/hrbridge/$(PROJECT)
	@install -m 0600 packaging/hrbridge.conf $(PACKAGE_DIR)/$(ARCH)/data/opt/etc/hrbridge/hrbridge.conf
	@install -m 0755 packaging/S99hrbridge $(PACKAGE_DIR)/$(ARCH)/data/opt/etc/init.d/S99hrbridge
	@sed -e 's/@VERSION@/$(VERSION)/g' -e 's/@ARCH@/$(ARCH)/g' packaging/control > $(PACKAGE_DIR)/$(ARCH)/control/control
	@install -m 0644 packaging/conffiles $(PACKAGE_DIR)/$(ARCH)/control/conffiles
	@printf "2.0\n" > $(PACKAGE_DIR)/$(ARCH)/debian-binary
	@cd $(PACKAGE_DIR)/$(ARCH)/control && COPYFILE_DISABLE=1 tar --uid 0 --gid 0 --uname root --gname root -czf ../control.tar.gz .
	@cd $(PACKAGE_DIR)/$(ARCH)/data && COPYFILE_DISABLE=1 tar --uid 0 --gid 0 --uname root --gname root -czf ../data.tar.gz .
	@rm -f $(BUILD_DIR)/$(PROJECT)_$(VERSION)_$(ARCH).ipk
	@cd $(PACKAGE_DIR)/$(ARCH) && COPYFILE_DISABLE=1 tar --uid 0 --gid 0 --uname root --gname root -czf ../../$(PROJECT)_$(VERSION)_$(ARCH).ipk ./debian-binary ./control.tar.gz ./data.tar.gz
	@gzip -dc $(BUILD_DIR)/$(PROJECT)_$(VERSION)_$(ARCH).ipk | tar -tf - | grep -qx './debian-binary'
	@gzip -dc $(BUILD_DIR)/$(PROJECT)_$(VERSION)_$(ARCH).ipk | tar -tf - | grep -qx './control.tar.gz'
	@gzip -dc $(BUILD_DIR)/$(PROJECT)_$(VERSION)_$(ARCH).ipk | tar -tf - | grep -qx './data.tar.gz'

clean:
	rm -rf $(BUILD_DIR)
