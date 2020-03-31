# Build uuid-annotator
FROM golang:1.14-alpine as build
RUN apk --no-cache add git
COPY . /go/src/github.com/m-lab/uuid-annotator
WORKDIR /go/src/github.com/m-lab/uuid-annotator
RUN go get -v \
      -ldflags "-X github.com/m-lab/go/prometheusx.GitShortCommit=$(git log -1 --format=%h)" \
      .

# Put it in its own image.
FROM alpine
COPY --from=build /go/bin/uuid-annotator /uuid-annotator
COPY ./data/asnames.ipinfo.csv /data/asnames.ipinfo.csv
ENV SITEINFO_URL /data/asnames.ipinfo.csv
WORKDIR /
ENTRYPOINT ["/uuid-annotator"]
