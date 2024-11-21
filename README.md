# tears

[![Go Reference](https://pkg.go.dev/badge/github.com/mariuswilms/tears.svg)](https://pkg.go.dev/github.com/mariuswilms/tears)

tears provides an elegant way to register cleanup functions, which are then called
when the program exits. Unlike defer, it adds handling for several cleanup
approaches, such as quit channels and similar mechanisms, and it offers more
robustness against deadlocks and ensures proper resource teardown. The package
has no dependencies outside of the standard library.

## Usage

Commonly resources used throughout the lifetime of a mid-to-smallish program are set up within the main function.
When the program exits, these resources need to be cleaned up. This is where tears comes in.

```go
func main() {
    tear, down := tears.New()

    conn, _ := net.Dial("tcp", "example.com:80")
    tear(conn.Close)

    // ...
    down(ctx)
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
    s.Down(context.Background())
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
    
    // ...
    down(context.Background())
}
```

Usually cleanup functions are called in a FIFO order. If you need to break out of that, tears
provides a way to prioritize cleanup functions.

```go
tear(conn.Close).End()
```

It's also possible to hook shutdown functions of another setup process into the main one.

```go
// main.go
func main() {
    tear, down := tears.New()

    otel, shutdown := setupOtel()
    tear(shutdown)

    // ...
    down(ctx)
}

// otel.go
func setupOtel() (tears.DownFn) {
    tear, down := tears.New()

    tear(/* ... */)

    return down
}
```
