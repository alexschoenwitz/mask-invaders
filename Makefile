generate:
	@buf generate 

fix:
	@buf format -w

run-server: generate
	@go run ./server/...
