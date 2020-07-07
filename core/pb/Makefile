# message.proto containes only message definitions. So no plugins required.
# also, it does not import any other proto spec. So no proto_path needed here.
PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)

SRC_REL = 'paths=source_relative:.'

all: $(GO)

%.pb.go: %.proto
	protoc --gogo_out=$(SRC_REL) $<

clean:
	rm -f *.pb.go
	rm -f *.go

