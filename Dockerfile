# use : docker build --build-arg PLUGIN_URL=https://github.com/meshplus/pier-client-fabric --build-arg PLUGIN_VERSION=v1.0.0-rc2 -t pier:1.0.0 .
FROM golang:1.14.2 as builder

RUN mkdir -p /go/src/github.com/meshplus/pier
WORKDIR /go/src/github.com/meshplus/pier

# Cache dependencies
COPY go.mod .
COPY go.sum .

RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download -x

# Build real binaries
COPY . .

RUN go get -u github.com/gobuffalo/packr/packr

RUN make install

RUN cd .. && \
    git clone https://github.com/meshplus/pier-client-fabric.git && \
    cd pier-client-fabric && \
    make fabric1.4 && \
    cp build/fabric-client-1.4.so /go/bin/fabric-client-1.4.so

#RUN cd .. && \
#    git clone https://github.com/meshplus/pier-client-ethereum.git && \
#    cd pier-client-ethereum && \
#    make eth && \
#    cp build/eth-client.so /go/bin/eth-client.so

# Final image
FROM frolvlad/alpine-glibc

WORKDIR /root

# Copy over binaries from the builder
COPY --from=builder /go/bin/pier /usr/local/bin
COPY ./build/libwasmer.so /lib
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/lib

RUN ["pier", "init"]

RUN mkdir -p /root/.pier/plugins
COPY --from=builder /go/bin/*.so /root/.pier/plugins/
COPY scripts/docker_entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker_entrypoint.sh

ENV APPCHAIN_NAME=fabric-client-1.4.so \
    PLUGIN_CONFIG=fabric

EXPOSE 44555 44544

# run pier inside container using:
# docker run -t --env APPCHAIN_NAME=fabric-client-1.4.so \
# --env PLUGIN_CONFIG=fabric \
# -v your/path/to/rule:/root/.pier/validating.wasm \
# -v your/path/to/pier.toml:/root/.pier/pier.toml \
# -v your/path/to/plugin_config:/root/.pier/fabric \
# pier:1.0.0

ENTRYPOINT docker_entrypoint.sh, "$APPCHAIN_NAME", "$PLUGIN_CONFIG"
