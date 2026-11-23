### Fixed

- Qoder SSE stream no longer leaks a goroutine per aborted stream. The pipe writer now aborts writes when the request context is done, so `qoderSSERewrite` exits when the client disconnects instead of blocking forever.

### Validation

- `go vet ./internal/proxy/executor/`, `go build ./internal/proxy/executor/`
