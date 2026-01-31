generate:
	@buf generate 

fix:
	@buf format -w

run-server: generate
	@go run ./server/...

run-ui: generate
	@go run ./ui-engine/...

run-game:
	@go run ./client/dumb-bot/main.go &
	@go run ./client/very-clever-bot/main.go &
