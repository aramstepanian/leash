.PHONY: leash test install assets app stop

leash:
	mkdir -p bin
	go build -o bin/leash ./cmd/leash

test:
	go test ./...
	go test -race ./...

assets:
	swift macos/render-assets.swift

# Stop the menu and whatever is bound to :17332. Do not pkill -f a
# pattern that appears in this recipe — that matches make itself.
stop:
	-@~/.leash/bin/leash stop 2>/dev/null || true
	-@./bin/leash stop 2>/dev/null || true
	-@killall Leash 2>/dev/null || true
	-@lsof -tiTCP:17332 -sTCP:LISTEN | xargs kill 2>/dev/null || true

install: leash
	@$(MAKE) stop
	mkdir -p "$(HOME)/.leash/bin"
	install -m 755 bin/leash "$(HOME)/.leash/bin/leash"
	"$(HOME)/.leash/bin/leash" install
	@echo ""
	@echo "Helper: $$($(HOME)/.leash/bin/leash version)"
	@echo "That is the daemon, not the menu. The Agent chips live in Leash.app."
	@echo "Rebuild it with:  make app"

app: install
	@if [ "$$(uname)" != "Darwin" ]; then echo "make app is Mac-only (xcodebuild)."; exit 1; fi
	xcodebuild -project macos/Leash.xcodeproj -scheme Leash -configuration Debug -derivedDataPath macos/.derived
	-@killall Leash 2>/dev/null || true
	open macos/.derived/Build/Products/Debug/Leash.app
	@echo "Opened this tree's Leash.app. Wordmark should read LEASH 0.9 — if it still says LEASH, you are looking at the old app."
