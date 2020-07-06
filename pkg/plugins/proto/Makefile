all: pb grpc
pb:
	protoc -I=. \
	-I${GOPATH}/src \
	-I${GOPATH}/src/github.com/gogo/protobuf/protobuf \
	--gogofast_out=:. \
	plugin.proto

grpc:
	protoc -I=. \
	-I=${GOPATH}/src \
	-I=${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
	-I=${GOPATH}/src/github.com/gogo/protobuf/protobuf \
	--grpc-gateway_out=logtostderr=true:. \
	--swagger_out=logtostderr=true:. \
	--gogofast_out=plugins=grpc:. \
	plugin.proto