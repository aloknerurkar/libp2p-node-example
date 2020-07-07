FROM golang:1.12-stretch
MAINTAINER Alok Nerurkar <aloknerurkar@gmail.com>

# Install protobuf related stuff
ENV PROTOBUF_VERSION=3.5.1 \
    PROTOC_GEN_DOC_VERSION=1.0.0-rc \
    SRC_DIR=/gopcp \
    PROTO_PATH=/protobuf

RUN mkdir -p $PROTO_PATH && \
    curl -L https://github.com/google/protobuf/archive/v${PROTOBUF_VERSION}.tar.gz \
    | tar xvz --strip-components=1 -C $PROTO_PATH

RUN apt-get update && apt-get install -y autoconf libtool && \
    cd $PROTO_PATH && \
    autoreconf -f -i -Wall,no-obsolete && \
    ./configure --prefix=/usr --enable-static=no && \
    make -j2 && make install

RUN cd $PROTO_PATH && \
    make install

#RUN find ${OUTDIR} -name "*.a" -delete -or -name "*.la" -delete

RUN mkdir -p $PROTO_PATH/google/protobuf && \
        for f in any duration descriptor empty struct timestamp wrappers; do \
            curl -L -o $PROTO_PATH/google/protobuf/${f}.proto \
            https://raw.githubusercontent.com/google/protobuf/master/src/google/protobuf/${f}.proto; \
        done && \
    mkdir -p $PROTO_PATH/google/api && \
        for f in annotations http; do \
            curl -L -o $PROTO_PATH/google/api/${f}.proto \
            https://raw.githubusercontent.com/grpc-ecosystem/grpc-gateway/master/third_party/googleapis/google/api/${f}.proto; \
        done && \
    mkdir -p $PROTO_PATH/github.com/gogo/protobuf/gogoproto && \
        curl -L -o $PROTO_PATH/github.com/gogo/protobuf/gogoproto/gogo.proto \
        https://raw.githubusercontent.com/gogo/protobuf/master/gogoproto/gogo.proto

RUN go get -u -v -ldflags '-w -s' \
	github.com/golang/protobuf/protoc-gen-go \
   github.com/gogo/protobuf/protoc-gen-gogo \
	github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger \
	github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway \
	&& install -c ${GOPATH}/bin/protoc-gen* /usr/bin/


# Download packages first so they can be cached.
COPY go.mod go.sum $SRC_DIR/
RUN cd $SRC_DIR \
  && go mod download

COPY . $SRC_DIR

ENV PROTO_PATH=/protobuf:$SRC_DIR:. \
    GOOGLE_API=/protobuf

# Build the thing.
# Also: fix getting HEAD commit hash via git rev-parse.
RUN cd $SRC_DIR \
  && make -e all \
  && go build

# Now comes the actual target image, which aims to be as small as possible.
FROM busybox:1-glibc
MAINTAINER Alok Nerurkar <aloknerurkar@gmail.com>

# Get the ipfs binary, entrypoint script, and TLS CAs from the build container.
ENV SRC_DIR /gopcp
COPY --from=0 $SRC_DIR/gopcp /usr/local/bin/gopcp
#COPY --from=0 $SRC_DIR/bin/container_daemon /usr/local/bin/start_gopcp
#COPY --from=0 /tmp/su-exec/su-exec /sbin/su-exec
#COPY --from=0 /tmp/tini /sbin/tini
#COPY --from=0 /etc/ssl/certs /etc/ssl/certs

# This shared lib (part of glibc) doesn't seem to be included with busybox.
COPY --from=0 /lib/x86_64-linux-gnu/libdl-2.24.so /lib/libdl.so.2

# Swarm TCP; should be exposed to the public
EXPOSE 4001
EXPOSE 10000
EXPOSE 10001
EXPOSE 10002

# Create the fs-repo directory and switch to a non-privileged user.
#ENV IPFS_PATH /data/ipfs
#RUN mkdir -p $IPFS_PATH \
#  && adduser -D -h $IPFS_PATH -u 1000 -G users ipfs \
#  && chown ipfs:users $IPFS_PATH

# Expose the fs-repo as a volume.
# start_ipfs initializes an fs-repo if none is mounted.
# Important this happens after the USER directive so permission are correct.
#VOLUME $IPFS_PATH

# The default logging level
#ENV IPFS_LOGGING ""

# This just makes sure that:
# 1. There's an fs-repo, and initializes one if there isn't.
# 2. The API and Gateway are accessible from outside the container.
ENTRYPOINT ["/usr/local/bin/gopcp"]

# Execute the daemon subcommand by default
CMD ["daemon"]
