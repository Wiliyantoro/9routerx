### Fixed

- Rate limiter no longer truncates chunked request bodies over 4 MiB. The TPM token scan now reassembles the scanned prefix with the remaining reader, so large chunked uploads (base64/multimodal payloads) reach the handler intact instead of answering 400.

### Validation

- `go vet ./internal/middleware/`, `go build ./internal/middleware/`
