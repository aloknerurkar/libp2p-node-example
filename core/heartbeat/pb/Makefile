# heartbeat.proto has only message definitions. So no plugins required.
# However it needs message.proto
PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)

# This has to be overridden. After go mod.
PROTO_PATH = $(GOPATH)/src:.
SRC_REL = 'paths=source_relative:.'

all: $(GO)

%.pb.go: %.proto
	protoc --proto_path=$(PROTO_PATH) --gogo_out=$(SRC_REL) $<

clean:
	rm -f *.pb.go
	rm -f *.go

