OSES=darwin linux
APPVERSION=$(shell cat VERSION)
BINPREFIX=multipull-$(APPVERSION)_
GO_LDFLAGS=-ldflags "-s -w -X .AppVersion=$(APPVERSION)"

multipull:
	CGO_ENABLED=0 go build $(GO_LDFLAGS)

clean:
	@if [ -d dist ]; then rm -Rf dist; fi
	go clean
.PHONY: clean

release-binaries: release-binary-windows
	@for os in $(OSES); do \
		echo "Building for $$os"; \
		CGO_ENABLED=0 GOARCH=amd64 GOOS=$$os go build $(GO_LDFLAGS) -o dist/$(BINPREFIX)$$os-amd64; \
		cd dist; \
		tar cfJ $(BINPREFIX)$$os-amd64.tar.xz $(BINPREFIX)$$os-amd64 && \
		    sha512sum $(BINPREFIX)$$os-amd64.tar.xz > $(BINPREFIX)$$os-amd64.tar.xz.sha512; \
		cd ..; \
	done
.PHONY: release-binaries

release-binary-windows:
	@if [ ! -d dist ]; then mkdir dist; fi
	@echo "Building for windows"
	@CGO_ENABLED=0 GOARCH=amd64 GOOS=windows go build $(GO_LDFLAGS) -o dist/$(BINPREFIX)windows-amd64.exe
	@cd dist && \
        tar cfJ $(BINPREFIX)windows-amd64.tar.xz $(BINPREFIX)windows-amd64.exe && \
        sha512sum $(BINPREFIX)windows-amd64.tar.xz > $(BINPREFIX)windows-amd64.tar.xz.sha512 && \
        cd ..
.PHONY: release-binary-windows
