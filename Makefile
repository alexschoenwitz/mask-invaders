generate:
	@protoc -I. --go_out=paths=source_relative:. server/api/api.proto 
