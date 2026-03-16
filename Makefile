BINARY   = sshmux
CMD      = ./cmd/sshmux
DESTDIR  = $(HOME)/bin

.PHONY: build install uninstall test fmt vet

build:
	go build -o $(BINARY) $(CMD)

install: build
	@mkdir -p $(DESTDIR)
	cp -f $(BINARY) $(DESTDIR)/$(BINARY)
	chmod 755 $(DESTDIR)/$(BINARY)
	@echo "Installed $(DESTDIR)/$(BINARY)"
	@echo "Importing SSH hosts from ~/.ssh/config ..."
	@$(DESTDIR)/$(BINARY) import-hosts 2>/dev/null && echo "SSH hosts imported" || echo "  (skipped -- no ~/.ssh/config)"
	@echo ""
	@echo "Done. Run: sshmux"

uninstall:
	rm -f $(DESTDIR)/$(BINARY)
	@echo "Removed $(DESTDIR)/$(BINARY)"

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...
