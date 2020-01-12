FROM golang:1.13.6-buster as base

ARG BASE=.
ENV BASE=$BASE

WORKDIR /out
WORKDIR /work

COPY $BASE/go.mod .
COPY $BASE/go.sum .

RUN go mod download

COPY $BASE/api api
COPY $BASE/cmd cmd
COPY $BASE/pkg pkg

RUN cd $BASE/cmd/bootstrap && go build -o /out/bootstrap .

FROM gcr.io/distroless/static:nonroot
COPY --from=base /out/bootstrap /bootstrap
ENTRYPOINT [ "/bootstrap" ]