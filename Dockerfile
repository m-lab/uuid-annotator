# Build uuid-annotator
FROM golang:1.13-alpine as build
RUN apk --no-cache add git
COPY . /go/src/github.com/m-lab/uuid-annotator
WORKDIR /go/src/github.com/m-lab/uuid-annotator
RUN go get -v \
      -ldflags "-X github.com/m-lab/go/prometheusx.GitShortCommit=$(git log -1 --format=%h)" \
      .
RUN chmod a+rx /go/bin/uuid-annotator

# Put it in its own image.
FROM alpine
COPY --from=build /go/bin/uuid-annotator /uuid-annotator
WORKDIR /
ENTRYPOINT ["/uuid-annotator"]
