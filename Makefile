.PHONY: leash test install

leash:
	mkdir -p bin
	go build -o bin/leash ./cmd/leash

test:
	go test ./...
	go test -race ./...

install: leash
	install -m 755 bin/leash "$(HOME)/.leash/bin/leash"
	"$(HOME)/.leash/bin/leash" install
	@echo "Start the daemon with: ~/.leash/bin/leash serve"
