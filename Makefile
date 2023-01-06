prepare:
	go install \
        github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
        github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
        google.golang.org/protobuf/cmd/protoc-gen-go \
        google.golang.org/grpc/cmd/protoc-gen-go-grpc

gen:
	mkdir -p pkg/pb && protoc -I=./proto \
		--go_opt paths=source_relative --go_out=pkg/pb \
		--go-grpc_opt paths=source_relative --go-grpc_out=pkg/pb \
		--grpc-gateway_opt paths=source_relative --grpc-gateway_out=pkg/pb \
		--openapiv2_opt logtostderr=true,allow_merge=true,enums_as_ints=true,openapi_naming_strategy=fqn  --openapiv2_out=docs \
		 ./proto/*.proto

test:
	go clean -testcache && go test -v -cover -failfast ./pkg/**/

integration-test:
	docker-compose -f docker-compose.yaml up --build
	# --abort-on-container-exit

clear-integration-test:
	docker-compose -f docker-compose.yaml down --volumes --remove-orphans --rmi local
