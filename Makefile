.PHONY: leash test install assets

leash:
	mkdir -p bin
	go build -o bin/leash ./cmd/leash

test:
	go test ./...
	go test -race ./...

assets:
	swift macos/render-assets.swift

install: leash
	mkdir -p "$(HOME)/.leash/bin"
	install -m 755 bin/leash "$(HOME)/.leash/bin/leash"
	"$(HOME)/.leash/bin/leash" install
	@echo "Start the daemon with: ~/.leash/bin/leash serve"
