.PHONY: build

install-js:
	pnpm install

assets: install-js
	node scripts/copy-assets.js

generate:
	templ generate

lint:
	golangci-lint run ./...

build: lint assets generate
	go build -o bin/kv .

run: assets generate
	go run .

dev: assets
	go tool air
