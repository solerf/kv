.PHONY: generate build run install assets dev

install:
	pnpm install

assets: install
	node scripts/copy-assets.js

generate:
	templ generate

build: assets generate
	go build -o bin/kv .

run: assets generate
	go run .

dev: assets
	go tool air
