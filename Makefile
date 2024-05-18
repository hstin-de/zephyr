
build-protobuf:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative protobuf/rpc.proto

build-prod:
	CGO_ENABLED=1 go build -v -ldflags='-s -w' -tags "osusergo" -gcflags="all=-trimpath=$(go env GOPATH)" -asmflags="all=-trimpath=$(go env GOPATH)" -trimpath -o ./build/zephyr