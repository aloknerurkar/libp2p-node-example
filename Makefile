SUBDIRS := core/pb core/heartbeat/pb gateway/pb

PROTO_PATH=$(PWD):.
export PROTO_PATH

.PHONY : all $(SUBDIRS)
all : $(SUBDIRS)

$(SUBDIRS) :
	$(MAKE) -e -C $@ clean all
