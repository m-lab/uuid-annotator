# UUID-Annotator

[![Version](https://img.shields.io/github/tag/m-lab/uuid-annotator.svg)](https://github.com/m-lab/uuid-annotator/releases)  [![Build Status](https://travis-ci.com/m-lab/uuid-annotator.svg?branch=master)](https://travis-ci.com/m-lab/uuid-annotator)  [![Coverage Status](https://coveralls.io/repos/m-lab/uuid-annotator/badge.svg?branch=master)](https://coveralls.io/github/m-lab/uuid-annotator?branch=master)  [![GoDoc](https://godoc.org/github.com/m-lab/uuid-annotator?status.svg)](https://godoc.org/github.com/m-lab/uuid-annotator)  [![Go Report Card](https://goreportcard.com/badge/github.com/m-lab/uuid-annotator)](https://goreportcard.com/report/github.com/m-lab/uuid-annotator)

A system for generating and saving per-connection metadata in real-time on
M-Lab's edge systems.

## Design

It generates a JSON file for every connection containing the geolocation and
network location metadata for the IP addresses in the connection, and eventually
adds in all other annotations concerning the "local environment" as well.

The datatype it generates will be "annotation" and it will generate filenames
like:

```txt
    /ndt/annotation/2009/03/18/${UUID}.json
```

where `${UUID}` is the actual UUID of the connection under consideration. in keeping
with both our uniform names best-practices and pusher best-practices.

The columns in the JSON file will initially be a subset of our standard columns:

- `client.Geo.*`
- `server.Geo.*`
- `client.Network.ASNumber`
- `server.Network.ASNumber`

Later versions can (and should!) add columns that include real-time switch
counters, local machine load, and other indicators of measurement quality,
but v1 will concentrate on location data. Each new column added to the
annotator output should be added to our set of standard columns.

The location annotation service will read from a MaxMind file served up via a
file stored in a GCS bucket. It will periodically poll (in a memoryless manner)
to discover whether the file has changed.

## Performance

This service will depend on tcp-info's UUID notification service, but no
local service should depend on the annotator. As such, we do not need to
worry about the annotator slowing down an integrated service, we only need to
worry about the annotator keeping up with the creation rate of TCP
connections. We do not anticipate that being too difficult.

## Availability

This service is a core service and needs to be highly available, just like
tcp-info, packet-headers, traceroute-caller, and DISCO. It represents our one
chance to annotate UUIDs with metadata. As such, the health of the experiment
service should depend on the health of the UUID annotation service, just like it
should depend on the other core services.
