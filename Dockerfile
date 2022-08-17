# Build uuid-annotator
FROM golang:1.18 as build
COPY . /go/src/github.com/m-lab/uuid-annotator
WORKDIR /go/src/github.com/m-lab/uuid-annotator
RUN go get -v . && \
    CGO_ENABLED=0 go install -v \
      -ldflags "-X github.com/m-lab/go/prometheusx.GitShortCommit=$(git log -1 --format=%h)" \
      .

# Put it in its own image.
FROM alpine
COPY --from=build /go/bin/uuid-annotator /uuid-annotator
COPY ./data/asnames.ipinfo.csv /data/asnames.ipinfo.csv
# In the fullness of time, we would like to replace this local file with a
# download from a GCS url that we control and is passed in as a command-line
# parameter, just like we do with the IP->AS mapping and the IP->geo mapping.
# For now, to prove to ourselves and IPInfo.io that doing that work might be
# worth it, we ship the 3.7MB data file with the binary.
ENV ASNAME_URL file:///data/asnames.ipinfo.csv
WORKDIR /
# Make sure /uuid-annotator can run (has no missing external dependencies).
RUN /uuid-annotator -h 2> /dev/null
ENTRYPOINT ["/uuid-annotator"]
