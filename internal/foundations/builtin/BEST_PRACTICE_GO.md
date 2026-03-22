# Go Best Practices

## Error Handling

- Always wrap errors with context: `fmt.Errorf("loading config: %w", err)`
- Check errors immediately after the call
- Use sentinel errors for expected conditions

## Nil Safety

- Check for nil before dereferencing pointers
- Guard against division by zero

## Concurrency

- Use sync.RWMutex for shared state
- Prefer channels for coordination between goroutines
- Capture channels before unlock to avoid races

## Testing

- Table-driven tests with descriptive names
- Race detector enabled: `go test -race`
- Tests alongside implementation in `_test.go` files
