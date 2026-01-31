generate:
	@buf generate 

fix:
	@buf format -w

run-server:
	@go run ./server/...

run-ui:
	@go run ./ui-engine/...

run-game:
	@go run ./client/dumb-bot/main.go &
	@go run ./client/very-clever-bot/main.go &
