install-plugins:
	@mkdir -p .bin
	@GOBIN=$(PWD)/.bin go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.27.7

generate: install-plugins
	@PATH=$(PWD)/.bin:$$PATH buf generate 

fix:
	@buf format -w

run-server: generate
	@go run ./server/...

run-ui: generate
	@go run ./ui-engine/...

run-ui2: generate
	@go run ./ui-engine-v2/...

run-game: generate
	$(eval GAME_ID := $(shell curl -s -X POST http://localhost:8080/v1/games | jq -r '.gameId'))
	@echo "Generated Game ID: $(GAME_ID)"
	@trap 'kill 0' SIGINT; \
	go run ./client/very-clever-bot/main.go -g $(GAME_ID) & \
	go run ./client/very-clever-bot/main.go -g $(GAME_ID) & \
	sleep 1 && go run ./client/very-clever-bot/main.go -s -g $(GAME_ID) & \
	wait
