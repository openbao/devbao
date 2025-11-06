GO?=go

.PHONY: bin
bin:
	rm -f bin/devbao
	mkdir -p bin/
	$(GO) build -o bin/devbao github.com/openbao/devbao/cmd/devbao
	ls -lh bin/devbao

.PHONY: fmt
fmt:
	$(GO) run mvdan.cc/gofumpt@latest -w -l $$(find . -name "*.go")

.PHONY: ci-fmt
ci-fmt:
	if [[ -n "$(shell $(GO) run mvdan.cc/gofumpt@latest -l $$(find . -name "*.go"))" ]]; then \
		echo "Formatting is not correct:" 1>&2 ; \
		$(GO) run mvdan.cc/gofumpt@latest -l -s $$(find . -name "*.go") ; \
		echo "" 1>&2 ; \
		echo "Run 'make fmt' to automatically fix this." 1>&2 ; \
		exit 1 ; \
	fi
