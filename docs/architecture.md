# Architecture and trust model

ClusterContract has three stages: strict input decoding and validation; pure evaluation at an explicit time; deterministic rendering. The NCCL adapter is a separate conversion step into the same normalized evidence model.

`contract.go` contains the public types, invariants, per-scope evaluation and verdict reduction. `json.go` bounds input size, rejects duplicate/unknown/case-altered keys and nulls, and then decodes typed values. `nccl.go` converts a documented text layout. `report.go` renders Markdown. The CLI handles files, flags and exit codes. Core evaluation has no network, process, filesystem, or wall-clock dependencies.

Evaluation indexes the newest record per scope/metric in O(E) time, expands and evaluates O(O) obligations plus selected sample scans, and sorts O obligations for reproducible output. It holds inputs and reports in memory; each serialized input is limited to 16 MiB. Independent ceilings bound 50,000 expanded obligations, 100,000 evidence records, 1,000,000 stored samples and 2,000,000 selected-sample comparisons. Each metadata string is limited to 1,024 bytes and descriptions to 4,096 bytes. Compact serialized obligation payload is charged before retaining each obligation and capped at 64 MiB; repeated provenance and waiver text count every time. Pretty JSON or Markdown can be larger. Exceeding a limit returns an error, with no partial report or partial acceptance. There is no scale benchmark or streaming claim.

## Conservative selection

The contract establishes an exact expected environment fingerprint and unit. Evidence selection first chooses the newest scope/metric record, then checks compatibility. This deliberately requires producers to separate distinct protocols into distinct metric names and supply a relevant bundle; a newer record from another environment produces incomplete coverage rather than allowing an older result to be cherry-picked. Equal scope/metric/timestamp records are ambiguous and rejected. Unrelated records are retained in the unused-evidence list.

There is no wildcard node expansion, inferred node membership, percentile selection, or implicit aggregation. For a collective metric, a communicator is a single explicit scope; no per-node performance conclusion follows from a collective aggregate. Evidence that individually covers all nodes must provide separate node records.

## Trust boundaries

The evaluator verifies the internal consistency of supplied documents. It cannot prove that a scope inventory is exhaustive, a timestamp is truthful, samples are representative, an environment digest was correctly constructed, a provenance reference is authentic, or a waiver owner authorized an exception. The contract, record bundle and evaluation time are all inputs that must be reviewed and retained together.

An optional SHA-256 field is a content identifier, not a signature. The NCCL adapter hashes raw input bytes; the normalized evaluator only validates a supplied hash's syntax. No external content is fetched. Contract and evidence descriptions are preserved so synthetic labels survive report generation. Markdown escapes markup and table delimiters; it does not turn provenance strings into executable links.

Waivers are visible policy decisions. Only a still-valid exact-scope waiver can replace a failed/incomplete obligation with `waived`; the original status and reasons remain. A conditional result cannot produce the successful CLI exit code. Expired and unused waivers remain visible for review.

## Hardware validation plan

1. Preserve complete stdout/stderr and process return status from pinned `all_reduce_perf` binaries, with command line, driver/CUDA/NCCL versions, communicator membership and inventory digest.
2. Capture repeat observations using an agreed warmup, repeat and message-size protocol. Contract thresholds must be agreed for the actual customer workload and hardware.
3. Exercise node omissions, firmware/configuration changes, failed correctness checks, transport failures and incomplete output; confirm none create acceptance.
4. Compare normalized records and reports against the source output and a human-reviewed acceptance decision on both single-node and multi-node hardware.
5. Publish the methodology, permitted evidence and limitations, including unexpected outcomes. Until that evidence exists, fixture tests are not GPU validation or proof of production readiness.
