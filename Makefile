generate:
	go tool gotailwind -i ./assets/css/input.css -o ./public/css/styles.css --minify
	go tool templ generate

update:
	go get -tool github.com/a-h/templ/cmd/templ@latest
	go get -tool github.com/hookenz/gotailwind/v4@latest
	go get -tool github.com/air-verse/air@latest
	go get -tool github.com/axzilla/templui/cmd/templui@latest

# Run templ generation in watch mode
templ:
	go tool templ generate --watch --proxy="http://localhost:8090" --open-browser=false

# Run air for Go hot reload
server:
	go tool air \
	--build.cmd "go build -o tmp/bin/main ./*.go" \
	--build.bin "tmp/bin/main" \
	--build.delay "100" \
	--build.exclude_dir "node_modules" \
	--build.include_ext "go" \
	--build.stop_on_error "false" \
	--misc.clean_on_exit true

tailwind-clean:
	go tool gotailwind -i ./assets/css/input.css -o ./public/css/styles.css --clean

tailwind-watch:
	go tool gotailwind -i ./assets/css/input.css -o ./public/css/styles.css --watch

dev:
	export DISCORD_CLIENT_ID=1107039927230791680 DISCORD_REDIRECT_URI=https://api.neuralnexus.dev/api/oauth TWITCH_CLIENT_ID=cx0nr5h65pexo8huupaywy08ry79pw TWITCH_REDIRECT_URI=https://api.neuralnexus.dev/api/oauth
	make tailwind-clean
	make -j3 tailwind-watch templ server

# --- Top-to-bottom test environment (containerized app + Playwright) ---

test-env-up:
	docker compose -f test/docker-compose.test.yml up -d --build --wait frontend

test-env-down:
	docker compose -f test/docker-compose.test.yml down -v

test-env-logs:
	docker compose -f test/docker-compose.test.yml logs -f

# Runs the Playwright suite against the real, containerized app - drives
# it in a real browser and mocks only the sibling API. Needs nothing but
# Docker: no local Node or browser install. Brings up the environment,
# runs the suite, then tears it down regardless of outcome.
test: test-env-up
	docker compose -f test/docker-compose.test.yml run --rm playwright; status=$$?; $(MAKE) test-env-down; exit $$status
