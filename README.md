## JumpCloud TakeHome

Hello :) This is my first Go application. I've attempted to make it as idiomatic
as possible, and have commented code inline to explain more specific reasoning.

### Build & Run

To build and run, from the project root:

`$ go build -o bin/jumpcloud-takehome && bin/jumpcloud-takehome`

The project should also be importable into IntelliJ GoLand.

### Design

At high level the service is designed around two concurrent command channels, one
for handling hashing requests, and one for handling statistics for those requests.

I opted for this pattern since making use of go routines and channels seems to be
the preferred Go way of handling concurrency. I also opted to keep the statistics
tracking a separate channel, because the instructions talk about stats for requests
to the POST endpoint, which I interpret to mean "all requests, even bad ones". For
example, I think it's important to capture a request count for the "/hash" endpoint
even if the HTTP method is wrong, or the HTTP Form in the body is malformed.

### Packages

This project is laid out in the following manner:

app/app.go:

Contains the HTTP handlers for the different endpoints. The handlers make use of the 
command channels aggregated by the App struct for executing the business logic of
the service.

hashes/hashes.go:

Contains the command loop to process changes to the hash store, via a command channel.
Also contains the logic for handling the graceful shutdown of in-flight hash requests.

middleware/middleware.go:

Generic HTTP middleware to be used by the app package to contain boilerplate like allowed
HTTP methods, or form parsing.

stats/stats.go:

Contains the command loop to process changes to the stats of the service, via a command channel.
Opted for this pattern so that business logic processing of the requests is separate from statistics
about those requests.