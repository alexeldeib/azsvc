FROM golang:1.13.6-buster as base

ENV KUSTOMIZE_VERSION=v3.5.4

WORKDIR /bin/kustomize

RUN go mod init kustomize
RUN go get sigs.k8s.io/kustomize/kustomize/v3@${KUSTOMIZE_VERSION}

FROM debian:buster

WORKDIR /

COPY --from=base /go/bin/kustomize /out/bin/kustomize
COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

ENTRYPOINT [ "/entrypoint.sh" ]