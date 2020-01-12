FROM golang:1.13.6-buster as base

WORKDIR /bin/bootstrap

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY api api
COPY pkg pkg
COPY cmd cmd

RUN ls

RUN cd ./cmd/bootstrap && go build -o /go/bin/bootstrap .

FROM debian:buster

WORKDIR /

COPY --from=base /go/bin/bootstrap /out/bin/bootstrap
COPY ./.github/actions/bootstrap/entrypoint.sh /entrypoint.sh

RUN chmod +x /out/bin/bootstrap
RUN chmod +x /entrypoint.sh

ENTRYPOINT [ "/out/bin/bootstrap" ]