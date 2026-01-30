generate:
	@buf generate 

fix:
	@buf format -w

run-server:
	@go run server/main.go
