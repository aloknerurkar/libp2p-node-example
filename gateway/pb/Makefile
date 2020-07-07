PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)
GOGW = $(PB:.proto=.pb.gw.go)
GOOGLE_API = $(GOPATH)/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis

all: $(GO) $(GOGW)

%.pb.go: %.proto
		protoc -I$(GOOGLE_API) --proto_path=$(PROTO_PATH) --gogo_out=plugins=grpc:. $<
%.pb.gw.go: %.proto
		protoc -I$(GOOGLE_API) --proto_path=$(PROTO_PATH) --grpc-gateway_out=logtostderr=true:. $<

clean:
		rm -f *.pb.go
		rm -f *.pb.gw.go
		rm -f *.go

