.PHONY: update generate vet dev build test test-go test-js

update:
	go get -tool github.com/a-h/templ/cmd/templ@latest
	go get -tool github.com/templui/templui/cmd/templui@latest
	go get -tool github.com/air-verse/air@latest
	go get -tool github.com/hookenz/gotailwind/v4@latest

generate:
	go tool gotailwind -i ./assets/css/input.css -o ./public/css/styles.css --minify
	go tool templ generate

# gofmt -l prints the files it would change, so the check fails when it prints anything
vet: generate
	@files="$$(gofmt -l .)"; if [ -n "$$files" ]; then echo "$$files"; exit 1; fi
	go tool templ fmt -fail components
	go vet ./...

dev: export NN_API_URL ?= http://0.0.0.0:8080
dev: export NN_SITE_URL ?= http://localhost:8090
dev: export DISCORD_CLIENT_ID ?= 1107039927230791680
dev: export DISCORD_REDIRECT_URI ?= https://api.neuralnexus.dev/api/oauth
dev: export TWITCH_CLIENT_ID ?= cx0nr5h65pexo8huupaywy08ry79pw
dev: export TWITCH_REDIRECT_URI ?= https://api.neuralnexus.dev/api/oauth
dev:
	go tool gotailwind -i ./assets/css/input.css -o ./public/css/styles.css --clean
	trap 'trap - INT TERM; kill 0' INT TERM; \
	go tool gotailwind -i ./assets/css/input.css -o ./public/css/styles.css --watch & \
	go tool templ generate --watch --proxy="http://localhost:8090" --open-browser=false & \
	go tool air \
		--build.cmd "go build -o tmp/bin/main ./*.go" \
		--build.bin "tmp/bin/main" \
		--build.delay "100" \
		--build.exclude_dir "node_modules" \
		--build.include_ext "go" \
		--build.stop_on_error "false" \
		--misc.clean_on_exit true & \
	wait

build: vet
	go build -o build/webserver .

test: test-go test-js

# config reads these at package init, so go test fails without them
test-go: export NN_API_URL = http://api.test
test-go: export NN_SITE_URL = http://site.test
test-go: generate
	go test -race ./...

# Runs the Playwright suite in Docker, tearing the environment down afterwards
test-js: compose = docker compose -f test/js/docker-compose.yml
test-js:
	$(compose) up -d --build --wait frontend && $(compose) run --rm playwright; status=$$?; $(compose) down -v; exit $$status
