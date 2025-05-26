generate:
	./tailwindcss -i ./assets/css/input.css -o ./public/css/styles.css --minify
	templ generate

downloadtw:
	curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
	chmod +x tailwindcss-linux-x64
	mv tailwindcss-linux-x64 ./tailwindcss

update:
	go install github.com/a-h/templ/cmd/templ@latest

	curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
	chmod +x tailwindcss-linux-x64
	mv tailwindcss-linux-x64 ./tailwindcss

	go install github.com/air-verse/air@latest

	go install github.com/axzilla/templui/cmd/templui@latest

# Run templ generation in watch mode
templ:
	templ generate --watch --proxy="http://localhost:8090" --open-browser=false

# Run air for Go hot reload
server:
	air \
	--build.cmd "go build -o tmp/bin/main ./*.go" \
	--build.bin "tmp/bin/main" \
	--build.delay "100" \
	--build.exclude_dir "node_modules" \
	--build.include_ext "go" \
	--build.stop_on_error "false" \
	--misc.clean_on_exit true

# Watch Tailwind CSS changes
tailwind:
	./tailwindcss -i ./assets/css/input.css -o ./public/css/styles.css --watch

# Start development server with all watchers
dev:
	export DISCORD_CLIENT_ID=1107039927230791680 DISCORD_REDIRECT_URI=https://api.neuralnexus.dev/api/oauth TWITCH_CLIENT_ID=cx0nr5h65pexo8huupaywy08ry79pw TWITCH_REDIRECT_URI=https://api.neuralnexus.dev/api/oauth
	make -j3 tailwind templ server
