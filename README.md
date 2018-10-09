# BlueWand SDK (Proof of Concept)


## Summary

This is a proof of concept using cross platform server/client architecture for bidirectional communication between Kano Coding Wand and Client connection.

[Protocol Buffers](https://developers.google.com/protocol-buffers/) is chosen for serialising data for its platform & language-neutral mechanism, which means 1 definition file, user can generate their own server/client, or simply reuse pre-compiled server and only generate the client written in user preferred programming language.

I also chose [Go](https://golang.org/) to write the sample server & client for its portability, you can simply generate a client in your preferred language and communicate to my prebuilt server.

Darwin (MacOS) is prebuilt and stored at the [/bin folder](https://github.com/anzellai/bluewand/tree/master/bin).
Linux plafform is being worked on due to dynamic linking, in coming releases we will use static linking so binary can be prebuilt.
Windows platform support will come in later releases, probably in FFI with native Windows DLL for Bluetooth LE support. No concrete timeline yet, but we will release this hopefully in early 2019.


### Built from source

The *proto* file is very simple and it is stored at `/bluewand/bluewand.proto`.
Go generate is written and it's in *Makefile*, simply run `make build` will build the gRPC library, the server and client binaries.


### Example Usage

Just run the server & client with your host platform under bin folder, the server and client should run on separate processes.

Server accepts environment flags for running "port" (default "55555"), also accepts flags like "duration", "tls" connection with "cert", "key" config.
Client accepts environment flags for connecting "server_addr" for server (default "127.0.0.1:55555"), also allows flags to use "tls" connection with "server_host_override" to set up your own host server.

There is already an example client built with Go to automatically connect and subscribe to OnButton and OnMotion RPC, you can play around by simply running the server and client executable in 2 separate terminals.

Open one terminal:

`./bin/darwin/bluewand-server`

Open another terminal:

`./bin/darwin/bluewand-client`


### TODO

Plenty, testing is a big thing we missed at the moment.
Also we would like to exposing more APIs from device protocol, communication message format / device set-mode, accepting more than 1 Wand instance, more robust definitions and more samples in different programming languages.

Contribution welcome.
