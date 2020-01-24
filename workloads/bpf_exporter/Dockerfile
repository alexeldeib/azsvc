ARG distro=xenial

FROM golang:1.13.6-stretch as builder

# Doing mostly what CI is doing here
RUN apt-get update && \
    apt-get install -y apt-transport-https && \
    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 648A4A16A23015EEF4A66B8E4052245BD4284CDD && \
    echo "deb https://repo.iovisor.org/apt/xenial xenial main" > /etc/apt/sources.list.d/iovisor.list && \
    apt-get update && \
    apt-get install -y linux-headers-amd64 && \
    curl -sL https://github.com/cloudflare/ebpf_exporter/files/3890546/libbcc_0.11.0-2_amd64.deb.gz | gunzip > /tmp/libbcc.deb && \
    dpkg -i /tmp/libbcc.deb

RUN git clone https://github.com/cloudflare/ebpf_exporter /go/ebpf_exporter

RUN cd /go/ebpf_exporter && GOPATH="" GOPROXY="off" GOFLAGS="-mod=vendor" go install -v ./...

FROM ubuntu:xenial

RUN apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 4052245BD4284CDD
RUN echo "deb http://repo.iovisor.org/apt/xenial xenial main" > /etc/apt/sources.list.d/iovisor.list
RUN apt update
RUN apt-get install -y bcc-tools libbcc-examples

COPY --from=builder /root/go/bin/ebpf_exporter /ebpf_exporter

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM ubuntu:xenial

# WORKDIR /
# COPY --from=base /bcc/build/src/cc/libbcc.a /usr/local/lib/libbcc.a
# COPY --from=base /bcc/build/src/cc/libbcc-loader-static.a /usr/local/lib/libbcc-loader-static.a
# COPY --from=base /bcc/build/src/cc/libbcc_bpf.a /usr/local/lib/libbpf.a

# USER nonroot

ENTRYPOINT ["/ebpf_exporter"]