# docconv

[![Go reference](https://pkg.go.dev/badge/code.sajari.com/docconv.svg)](https://pkg.go.dev/code.sajari.com/docconv)
[![Build status](https://github.com/sajari/docconv/workflows/Go/badge.svg?branch=master)](https://github.com/sajari/docconv/actions)
[![Report card](https://goreportcard.com/badge/code.sajari.com/docconv)](https://goreportcard.com/report/code.sajari.com/docconv)
[![Sourcegraph](https://sourcegraph.com/github.com/sajari/docconv/-/badge.svg)](https://sourcegraph.com/github.com/sajari/docconv)

A Go wrapper library to convert PDF, DOC, DOCX, XML, HTML, RTF, ODT, Pages documents and images (see optional dependencies below) to plain text.

> **Note for returning users:** the Go import path for this package changed to `code.sajari.com/docconv`.

## Installation

If you haven't setup Go before, you first need to [install Go](https://golang.org/doc/install).

To fetch and build the code:

    $ go get code.sajari.com/docconv/...

This will also build the command line tool `docd` into `$GOPATH/bin`. Make sure that `$GOPATH/bin` is in your `PATH` environment variable.

## Dependencies

tidy, wv, popplerutils, unrtf, https://github.com/JalfResi/justext

Example install of dependencies (not all systems):

    $ sudo apt-get install poppler-utils wv unrtf tidy
    $ go get github.com/JalfResi/justext

### Optional dependencies

To add image support to the `docconv` library you first need to [install and build gosseract](https://github.com/otiai10/gosseract/tree/v2.2.4).

Now you can add `-tags ocr` to any `go` command when building/fetching/testing `docconv` to include support for processing images:

    $ go get -tags ocr code.sajari.com/docconv/...

This may complain on macOS, which you can fix by installing [tesseract](https://tesseract-ocr.github.io) via brew:

    $ brew install tesseract

## docd tool

The `docd` tool runs as either:

1.  a service on port 8888 (by default)

    Documents can be sent as a multipart POST request and the plain text (body) and meta information are then returned as a JSON object.

2.  a service exposed from within a Docker container

    This also runs as a service, but from within a Docker container. There are three build scripts:

    - [./docd/debian.sh](./docd/debian.sh)
    - [./docd/alpine.sh](./docd/alpine.sh)
    - [./docd/appengine.sh](./docd/appengine.sh)

    The `debian` version uses the Debian package repository which can vary with builds. The `alpine` version uses a very cut down Linux distribution to produce a container ~40MB. It also locks the dependency versions for consistency, but may miss out on future updates. The `appengine` version is a flex based custom runtime for Google Cloud.

3.  via the command line.

    Documents can be sent as an argument, e.g.

        $ docd -input document.pdf

### Optional flags

- `addr` - the bind address for the HTTP server, default is ":8888"
- `log-level`
  - 0: errors & critical info
  - 1: inclues 0 and logs each request as well
  - 2: include 1 and logs the response payloads
- `readability-length-low` - sets the readability length low if the ?readability=1 parameter is set
- `readability-length-high` - sets the readability length high if the ?readability=1 parameter is set
- `readability-stopwords-low` - sets the readability stopwords low if the ?readability=1 parameter is set
- `readability-stopwords-high` - sets the readability stopwords high if the ?readability=1 parameter is set
- `readability-max-link-density` - sets the readability max link density if the ?readability=1 parameter is set
- `readability-max-heading-distance` - sets the readability max heading distance if the ?readability=1 parameter is set
- `readability-use-classes` - comma separated list of readability classes to use if the ?readability=1 parameter is set

### How to start the service

    $ # This will only log errors and critical info
    $ docd -log-level 0

    $ # This will run on port 8000 and log each request
    $ docd -addr :8000 -log-level 1

## Example usage (code)

Some basic code is shown below, but normally you would accept the file by HTTP or open it from the file system.

This should be enough to get you started though.

### Use case 1: run locally

> Note: this assumes you have the [dependencies](#dependencies) installed.

```go
package main

import (
	"fmt"
	"log"

	"code.sajari.com/docconv"
)

func main() {
	res, err := docconv.ConvertPath("your-file.pdf")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res)
}
```

### Use case 2: request over the network

```go
package main

import (
	"fmt"
	"log"

	"code.sajari.com/docconv/client"
)

func main() {
	// Create a new client, using the default endpoint (localhost:8888)
	c := client.New()

	res, err := client.ConvertPath(c, "your-file.pdf")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res)
}
```
