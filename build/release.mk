dist: | check-style test package

build-linux:
	@echo Build Linux amd64
ifeq ($(BUILDER_GOOS_GOARCH),"linux_amd64")
	env GOOS=linux GOARCH=amd64 $(GO) build -o $(GOBIN) $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./...
else
	mkdir -p $(GOBIN)/linux_amd64
	env GOOS=linux GOARCH=amd64 $(GO) build -o $(GOBIN)/linux_amd64 $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./...
endif

build-osx:
	@echo Build OSX amd64
ifeq ($(BUILDER_GOOS_GOARCH),"darwin_amd64")
	env GOOS=darwin GOARCH=amd64 $(GO) build -o $(GOBIN) $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./...
else
	mkdir -p $(GOBIN)/darwin_amd64
	env GOOS=darwin GOARCH=amd64 $(GO) build -o $(GOBIN)/darwin_amd64 $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./...
endif

build-windows:
	@echo Build Windows amd64
ifeq ($(BUILDER_GOOS_GOARCH),"windows_amd64")
	env GOOS=windows GOARCH=amd64 $(GO) build -o $(GOBIN) $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./...
else
	mkdir -p $(GOBIN)/windows_amd64
	env GOOS=windows GOARCH=amd64 $(GO) build -o $(GOBIN)/windows_amd64 $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./...
endif

build-cmd-linux:
	@echo Build Linux amd64
ifeq ($(BUILDER_GOOS_GOARCH),"linux_amd64")
	env GOOS=linux GOARCH=amd64 $(GO) build -o $(GOBIN) $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./cmd/...
else
	mkdir -p $(GOBIN)/linux_amd64
	env GOOS=linux GOARCH=amd64 $(GO) build -o $(GOBIN)/linux_amd64 $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./cmd/...
endif

build-cmd-osx:
	@echo Build OSX amd64
ifeq ($(BUILDER_GOOS_GOARCH),"darwin_amd64")
	env GOOS=darwin GOARCH=amd64 $(GO) build -o $(GOBIN) $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./cmd/...
else
	mkdir -p $(GOBIN)/darwin_amd64
	env GOOS=darwin GOARCH=amd64 $(GO) build -o $(GOBIN)/darwin_amd64 $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./cmd/...
endif

build-cmd-windows:
	@echo Build Windows amd64
ifeq ($(BUILDER_GOOS_GOARCH),"windows_amd64")
	env GOOS=windows GOARCH=amd64 $(GO) build -o $(GOBIN) $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./cmd/...
else
	mkdir -p $(GOBIN)/windows_amd64
	env GOOS=windows GOARCH=amd64 $(GO) build -o $(GOBIN)/windows_amd64 $(GOFLAGS) -trimpath -ldflags '$(LDFLAGS)' ./cmd/...
endif

build: build-linux build-windows build-osx

build-cmd: build-cmd-linux build-cmd-windows build-cmd-osx

build-client:
	@echo Building mattermost web app

	cd $(BUILD_WEBAPP_DIR) && $(MAKE) build

package-prep:
	@ echo Packaging mattermost
	@# Remove any old files
	rm -Rf $(DIST_ROOT)

	@# Resource directories
	mkdir -p $(DIST_PATH)/config
	cp -L config/README.md $(DIST_PATH)/config
	OUTPUT_CONFIG=$(PWD)/$(DIST_PATH)/config/config.json go run ./scripts/config_generator
	cp -RL fonts $(DIST_PATH)
	cp -RL templates $(DIST_PATH)
	rm -rf $(DIST_PATH)/templates/*.mjml $(DIST_PATH)/templates/partials/
	cp -RL i18n $(DIST_PATH)

	@# Disable developer settings
	sed -i'' -e 's|"ConsoleLevel": "DEBUG"|"ConsoleLevel": "INFO"|g' $(DIST_PATH)/config/config.json
	sed -i'' -e 's|"SiteURL": "http://localhost:8065"|"SiteURL": ""|g' $(DIST_PATH)/config/config.json

	@# Reset email sending to original configuration
	sed -i'' -e 's|"SendEmailNotifications": true,|"SendEmailNotifications": false,|g' $(DIST_PATH)/config/config.json
	sed -i'' -e 's|"FeedbackEmail": "test@example.com",|"FeedbackEmail": "",|g' $(DIST_PATH)/config/config.json
	sed -i'' -e 's|"ReplyToAddress": "test@example.com",|"ReplyToAddress": "",|g' $(DIST_PATH)/config/config.json
	sed -i'' -e 's|"SMTPServer": "localhost",|"SMTPServer": "",|g' $(DIST_PATH)/config/config.json
	sed -i'' -e 's|"SMTPPort": "2500",|"SMTPPort": "",|g' $(DIST_PATH)/config/config.json

	@# Package webapp
	mkdir -p $(DIST_PATH)/client
	cp -RL $(BUILD_WEBAPP_DIR)/dist/* $(DIST_PATH)/client

	@# Help files
ifeq ($(BUILD_ENTERPRISE_READY),true)
	cp $(BUILD_ENTERPRISE_DIR)/ENTERPRISE-EDITION-LICENSE.txt $(DIST_PATH)
	cp -L $(BUILD_ENTERPRISE_DIR)/cloud/config/cloud_defaults.json $(DIST_PATH)/config
else
	cp build/MIT-COMPILED-LICENSE.md $(DIST_PATH)
endif
	cp NOTICE.txt $(DIST_PATH)
	cp README.md $(DIST_PATH)
	if [ -f ../manifest.txt ]; then \
		cp ../manifest.txt $(DIST_PATH); \
	fi

	@# Import Mattermost plugin public key
	gpg --import build/plugin-production-public-key.gpg

	@# Download prepackaged plugins
	mkdir -p tmpprepackaged
	@cd tmpprepackaged && for plugin_package in $(PLUGIN_PACKAGES) ; do \
		for ARCH in "osx-amd64" "windows-amd64" "linux-amd64" ; do \
			curl -f -O -L https://plugins-store.test.mattermost.com/release/$$plugin_package-$$ARCH.tar.gz; \
			curl -f -O -L https://plugins-store.test.mattermost.com/release/$$plugin_package-$$ARCH.tar.gz.sig; \
		done; \
	done


package-osx: package-prep
	@# Create needed directories
	mkdir -p $(DIST_PATH_OSX)/bin
	mkdir -p $(DIST_PATH_OSX)/logs
	mkdir -p $(DIST_PATH_OSX)/prepackaged_plugins

	@# Copy binary
ifeq ($(BUILDER_GOOS_GOARCH),"darwin_amd64")
	cp $(GOBIN)/mattermost $(DIST_PATH_OSX)/bin # from native bin dir, not cross-compiled
else
	cp $(GOBIN)/darwin_amd64/mattermost $(DIST_PATH_OSX)/bin # from cross-compiled bin dir
endif
	#Download MMCTL for OSX
	scripts/download_mmctl_release.sh "Darwin" $(DIST_PATH_OSX)/bin
	@# Prepackage plugins
	@for plugin_package in $(PLUGIN_PACKAGES) ; do \
		ARCH="osx-amd64"; \
		cp tmpprepackaged/$$plugin_package-$$ARCH.tar.gz $(DIST_PATH_OSX)/prepackaged_plugins; \
		cp tmpprepackaged/$$plugin_package-$$ARCH.tar.gz.sig $(DIST_PATH_OSX)/prepackaged_plugins; \
		HAS_ARCH=`tar -tf $(DIST_PATH_OSX)/prepackaged_plugins/$$plugin_package-$$ARCH.tar.gz | grep -oE "dist/plugin-.*"`; \
		if [ "$$HAS_ARCH" != "dist/plugin-darwin-amd64" ]; then \
			echo "Contains $$HAS_ARCH in $$plugin_package-$$ARCH.tar.gz but needs dist/plugin-darwin-amd64"; \
			exit 1; \
		fi; \
		gpg --verify $(DIST_PATH_OSX)/prepackaged_plugins/$$plugin_package-$$ARCH.tar.gz.sig $(DIST_PATH_OSX)/prepackaged_plugins/$$plugin_package-$$ARCH.tar.gz; \
		if [ $$? -ne 0 ]; then \
			echo "Failed to verify $$plugin_package-$$ARCH.tar.gz|$$plugin_package-$$ARCH.tar.gz.sig"; \
			exit 1; \
		fi; \
	done
	@# Package
	tar -C $(DIST_PATH_OSX)/.. -czf $(DIST_PATH)-$(BUILD_TYPE_NAME)-osx-amd64.tar.gz mattermost ../mattermost
	@# Cleanup
	rm -rf $(DIST_PATH_OSX)

package-windows: package-prep
	@# Create needed directories
	mkdir -p $(DIST_PATH_WIN)/bin
	mkdir -p $(DIST_PATH_WIN)/logs
	mkdir -p $(DIST_PATH_WIN)/prepackaged_plugins

	@# Copy binary
ifeq ($(BUILDER_GOOS_GOARCH),"windows_amd64")
	cp $(GOBIN)/mattermost.exe $(DIST_PATH_WIN)/bin # from native bin dir, not cross-compiled
else
	cp $(GOBIN)/windows_amd64/mattermost.exe $(DIST_PATH_WIN)/bin # from cross-compiled bin dir
endif
	#Download MMCTL for Windows
	scripts/download_mmctl_release.sh "Windows" $(DIST_PATH_WIN)/bin
	@# Prepackage plugins
	@for plugin_package in $(PLUGIN_PACKAGES) ; do \
		ARCH="windows-amd64"; \
		cp tmpprepackaged/$$plugin_package-$$ARCH.tar.gz $(DIST_PATH_WIN)/prepackaged_plugins; \
		cp tmpprepackaged/$$plugin_package-$$ARCH.tar.gz.sig $(DIST_PATH_WIN)/prepackaged_plugins; \
		HAS_ARCH=`tar -tf $(DIST_PATH_WIN)/prepackaged_plugins/$$plugin_package-$$ARCH.tar.gz | grep -oE "dist/plugin-.*"`; \
		if [ "$$HAS_ARCH" != "dist/plugin-windows-amd64.exe" ]; then \
			echo "Contains $$HAS_ARCH in $$plugin_package-$$ARCH.tar.gz but needs dist/plugin-windows-amd64.exe"; \
			exit 1; \
		fi; \
		gpg --verify $(DIST_PATH_WIN)/prepackaged_plugins/$$plugin_package-$$ARCH.tar.gz.sig $(DIST_PATH_WIN)/prepackaged_plugins/$$plugin_package-$$ARCH.tar.gz; \
		if [ $$? -ne 0 ]; then \
			echo "Failed to verify $$plugin_package-$$ARCH.tar.gz|$$plugin_package-$$ARCH.tar.gz.sig"; \
			exit 1; \
		fi; \
	done
	@# Package
	cd $(DIST_PATH_WIN)/.. && zip -9 -r -q -l ../mattermost-$(BUILD_TYPE_NAME)-windows-amd64.zip mattermost ../mattermost && cd ../..
	@# Cleanup
	rm -rf $(DIST_PATH_WIN)

package-linux: package-prep
	@# Create needed directories
	mkdir -p $(DIST_PATH_LIN)/bin
	mkdir -p $(DIST_PATH_LIN)/logs
	mkdir -p $(DIST_PATH_LIN)/prepackaged_plugins

	@# Copy binary
ifeq ($(BUILDER_GOOS_GOARCH),"linux_amd64")
	cp $(GOBIN)/mattermost $(DIST_PATH_LIN)/bin # from native bin dir, not cross-compiled
else
	cp $(GOBIN)/linux_amd64/mattermost $(DIST_PATH_LIN)/bin # from cross-compiled bin dir
endif
	#Download MMCTL for Linux
	scripts/download_mmctl_release.sh "Linux" $(DIST_PATH_LIN)/bin
	@# Prepackage plugins
	@for plugin_package in $(PLUGIN_PACKAGES) ; do \
		ARCH="linux-amd64"; \
		cp tmpprepackaged/$$plugin_package-$$ARCH.tar.gz $(DIST_PATH_LIN)/prepackaged_plugins; \
		cp tmpprepackaged/$$plugin_package-$$ARCH.tar.gz.sig $(DIST_PATH_LIN)/prepackaged_plugins; \
		HAS_ARCH=`tar -tf $(DIST_PATH_LIN)/prepackaged_plugins/$$plugin_package-$$ARCH.tar.gz | grep -oE "dist/plugin-.*"`; \
		if [ "$$HAS_ARCH" != "dist/plugin-linux-amd64" ]; then \
			echo "Contains $$HAS_ARCH in $$plugin_package-$$ARCH.tar.gz but needs dist/plugin-linux-amd64"; \
			exit 1; \
		fi; \
		gpg --verify $(DIST_PATH_LIN)/prepackaged_plugins/$$plugin_package-$$ARCH.tar.gz.sig $(DIST_PATH_LIN)/prepackaged_plugins/$$plugin_package-$$ARCH.tar.gz; \
		if [ $$? -ne 0 ]; then \
			echo "Failed to verify $$plugin_package-$$ARCH.tar.gz|$$plugin_package-$$ARCH.tar.gz.sig"; \
			exit 1; \
		fi; \
	done
	@# Package
	tar -C $(DIST_PATH_LIN)/.. -czf $(DIST_PATH)-$(BUILD_TYPE_NAME)-linux-amd64.tar.gz mattermost ../mattermost
	@# Preserve native package so dev machines will have an unzipped package available
	mv $(DIST_PATH_LIN)/bin $(DIST_PATH)/bin
	@# Cleanup
	rm -rf $(DIST_PATH_LIN)

package: package-osx package-windows package-linux
	rm -rf tmpprepackaged
	rm -rf $(DIST_PATH)
