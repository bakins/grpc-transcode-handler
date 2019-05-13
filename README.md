# grpc-transcode-handler

Go HTTP server handler for transcoding HTTP+JSON requests to gRPC.

## Why?

I was doing a side project with [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) and decided to try something similar without code generation that would run in-process with the "real" gRPC server.

Also, for whatever reason, I like doing silly experiments with gRPC in Go.

## What is it?

`grpc-transcode-handler` is a Go package that allows one to create an HTTP handler that will transcode HTTP+JSON
to gRPC in-process.

`grpc-transcode-handler` does not allow one to override URLs for services.  The URL path will be the same as it is in gRPC. For example,
in the [helloworld](https://github.com/grpc/grpc-go/blob/master/examples/helloworld/helloworld/helloworld.proto) example, all methods in the
`helloworld` namespace for service `Greeter` would be at HTTP path `/helloworld.Greeter`, so the `SayHello` method would be at `/helloworld.Greeter/SayHello`.  If you run the [example server](./example/server) in this project, you could use curl to make a request like:

```
curl -XPOST -H "Content-Type: application/json" -d '{ "name": "world"}' -sv http://localhost:9090/helloworld.Greeter/SayHello
```

## How it works?

See the [example server](./example/server/server.go) for example usage.

The general flow while writing a server is:

* create a handler
* register a grpc service on the handler. You may also register this service on another "real" gRPC server.
* start the handler - this starts an in process gRPC server for the transcoding.

While running, the handler when it receives an HTTP request will

* Read the request body
* add grpc framing - the body is not processed in any way, yes
* send the grpc request over an [in memory listner](https://github.com/akutz/memconn) to the handlers internal gRPC server
* the internal gRPC server will unmarshal the JSON body to the appropriate structs and call your service handlers
* the internal gRPC server will marshal responses as JSON (with the proper gRPC framing) and send them over the in memory connection
* The client will receieve the response, send approriate HTTP headers, and send response to original client

Like [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway), `grpc-transcode-handler` handles streamed responses by outputting newline separated JSON. Streaming requests are not supported.

## Inspired by

* https://github.com/mwitkow/grpc-proxy - grpc passthrough proxy
* https://github.com/grpc-ecosystem/grpc-gateway

## License

see [LICENSE](./LICENSE)
