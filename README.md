# Chatroom
Chatroom is a multi-room chat application writing in Go and using Gorilla framework for websocket.

## Getting Started
This project requires a working Go development environment. The [Getting Started](http://golang.org/doc/install) page describes how to install the development environment.

Once you have Go up and running, you can download, build and run the example
using the following commands.

    $ go get github.com/korrawe/chatroom
    $ cd `go list -f '{{.Dir}}' github.com/korrawe/chatroom`
    $ go build && chatroom

To use the chat example, open http://localhost:8080/ in your browser.
