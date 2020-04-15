// Package ipservice sets up and queries and runs the RPC service for annotating IP addresses.
package ipservice

import "flag"

// SocketFilename is a flag to allow both clients and servers to use the same command-line flag.
var SocketFilename = flag.String(
	"ipservice.sock",
	"ipservice.sock",
	"The filename to use as a UNIX domain socket for the local annotation service.")
