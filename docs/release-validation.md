# Initial release validation

Validated October 4, 2026 using Go 1.27.1 on macOS arm64. This records software validation, not hardware or production validation.

- Independent implementation review, followed by regression checks for each identified false-pass/input-parsing issue.
- `go test -race -cover ./...` and `go vet ./...` passed on the reviewed source.
- Bundled synthetic examples produced the documented decisions and process exit codes.
- Linux/macOS amd64/arm64 archives were cross-built. Archive checksums, executable/license/example contents, and the extracted macOS arm64 executable were checked.
- CI repeats formatting, vet, tests and build on Ubuntu. Check the actual workflow run for its result; this document does not claim a future CI run passed.

The end-to-end NCCL fixture import produced a passing communicator assessment; adding customer obligations for unmeasured fabric/storage scopes produced incomplete; a correctness error failed despite adequate bandwidth; a scoped waiver produced conditional exit 3. Reversed placement headers and contradictory correctness summaries are rejected. All values are synthetic.

Neither the importer nor evaluator authenticates a producer, verifies hardware, or establishes statistical confidence. Live cluster integrations and operator usefulness remain to be validated.
