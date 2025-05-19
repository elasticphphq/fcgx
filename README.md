# fcgx

A minimal, robust, and modern FastCGI client library for Go.

**fcgx** is designed for integrating with PHP-FPM and other FastCGI servers, aiming for idiomatic Go, high testability, and correct protocol handling. It supports context, deadlines, timeouts, and structured error handling.

## Features

- Idiomatic, thread-safe Go API
- Context and timeout support on all requests
- Structured sentinel errors for robust error handling (`errors.Is`)
- Manual and reliable FastCGI protocol handling
- Designed for integration with PHP-FPM status, pool metrics, and more
- Well-suited for Kubernetes, Docker, and production monitoring

## Quick Example

```go
import (
    "context"
    "github.com/elasticphphq/fcgx"
    "io"
    "time"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    client, err := fcgx.DialContext(ctx, "unix", "/var/run/php-fpm.sock")
    if err != nil {
        panic(err)
    }
    defer client.Close()

    params := map[string]string{
        "SCRIPT_FILENAME": "/usr/share/phpmyadmin/index.php",
        "SCRIPT_NAME":     "/index.php",
        "REQUEST_METHOD":  "GET",
        "SERVER_PROTOCOL": "HTTP/1.1",
        "REMOTE_ADDR":     "127.0.0.1",
    }

    resp, err := client.Get(ctx, params)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    println(string(body))
}
```

## Error Handling

fcgx returns strong sentinel errors for key error categories:
- `ErrClientClosed`
- `ErrTimeout`
- `ErrContextCancelled`
- `ErrUnexpectedEOF`
- `ErrInvalidResponse`
- `ErrWrite`, `ErrRead`

Use `errors.Is` to match error causes in your code.

## Authors
* [Sylvester Damgaard](https://github.com/sylvesterdamgaard)

## License
MIT