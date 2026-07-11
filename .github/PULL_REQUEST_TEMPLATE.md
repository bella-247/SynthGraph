## What does this PR do?

<!-- One paragraph description of the change. -->

## Why is it needed?

<!-- Link to the issue this resolves, or explain the motivation. -->

Closes #

## How was it tested?

<!-- Describe the tests added or updated. Paste relevant test output if helpful. -->

## Does it change any existing behavior?

<!-- Yes / No. If yes, describe what changes and whether it is a breaking change. -->

## Checklist

- [ ] Tests added or updated
- [ ] `CGO_ENABLED=1 go test -count=1 ./...` passes
- [ ] `go vet ./...` passes
- [ ] No cross-stage imports (parser doesn't import generator, etc.)
- [ ] No `time.Now()` or unseeded randomness in the generation pipeline
- [ ] Error messages are clear and include position context where possible
- [ ] Exported functions have doc comments
