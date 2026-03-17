BINARY   = sshmux
CMD      = ./cmd/sshmux
DESTDIR  = $(HOME)/bin
REALDIR  = $(HOME)/.sshmux/bin
REALBIN  = $(REALDIR)/$(BINARY)-real

.PHONY: build install uninstall test fmt vet

build:
	go build -o $(BINARY) $(CMD)

install: build
	@mkdir -p $(DESTDIR) $(REALDIR)
	cp -f $(BINARY) $(REALBIN)
	chmod 755 $(REALBIN)
	@printf '%s\n' '#!/bin/sh' 'exec "$$HOME/.sshmux/bin/sshmux-real" "$$@"' > $(DESTDIR)/$(BINARY)
	chmod 755 $(DESTDIR)/$(BINARY)
	@echo "Installed wrapper $(DESTDIR)/$(BINARY)"
	@echo "Installed binary  $(REALBIN)"
	@echo ""
	@echo "Done. Run: sshmux"
	@echo "Tip: run 'sshmux import-hosts' in your terminal to import SSH hosts."

uninstall:
	rm -f $(DESTDIR)/$(BINARY)
	rm -f $(REALBIN)
	@echo "Removed $(DESTDIR)/$(BINARY)"
	@echo "Removed $(REALBIN)"

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...
