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
- [ ] `make test` passes (`go test ./... -race`)
- [ ] `make lint` passes (`golangci-lint run ./...`)
- [ ] No cross-stage imports (parser doesn't import generator, etc.)
- [ ] No `time.Now()` or unseeded randomness in the generation pipeline
- [ ] Error messages follow the format in SRS §12.2
- [ ] Exported functions have doc comments
- [ ] Golden files updated if output format changed (`make test-golden UPDATE=true`)
