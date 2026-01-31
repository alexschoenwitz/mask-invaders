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
	@go run ./client/very-clever-bot/main.go &
	@go run ./client/very-clever-bot/main.go &
	@go run ./client/very-clever-bot/main.go &
	@go run ./client/very-clever-bot/main.go &
	@go run ./client/very-clever-bot/main.go -s &
