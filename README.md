# tears

[![Go Reference](https://pkg.go.dev/badge/github.com/mariuswilms/tears.svg)](https://pkg.go.dev/github.com/mariuswilms/tears)

tears provides a way to register cleanup functions, which are than called when the program exits. Over defer it adds
handling of quit channels and alike, and provides more control over when the teardown of resources should happen. The package
has no dependencies outisde of the standard library.

## Usage

Commonly resources used throughout the lifetime of a mid-to-smallish program are set up within the main function. 
When the program exits, these resources need to be cleaned up. This is where tears comes in.

```go 
func main() {
    tear, down := tears.New()

    conn, _ := net.Dial("tcp", "example.com:80")
    tear(conn.Close)

    // ...
    down()
}
```

Over the defer statement tears has the advantage that cleanup tasks can be
registered in one method and later run in another method. This is commonly the
case if you separate the start and stop/close logic with methods on a struct.

```go
type Server struct {
    tears.Cleaner
    // ...
}

func (s *Server) Start() {
    s.Tear(lis.Close)
    // ...
}

func (s *Server) Stop() {
    s.Down()
}
```

Outside of calling a function directly, tears can help in 
closing go routines by using socalled quit-channels.

```go
func main() {
    tear, down := tears.New()

    quit := make(chan bool)
    tear(quit)

    go func() {
        select {
        // ...
        case <-quit:
            return
        }
    }

    down()
}
```
