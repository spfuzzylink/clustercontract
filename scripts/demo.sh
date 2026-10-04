#!/bin/sh
# All observations are invented: SYNTHETIC NOT GPU VALIDATION.
set -eu
cd "$(dirname "$0")/.."
clustercontract_demo_dir=$(mktemp -d)
trap 'rm -rf "$clustercontract_demo_dir"' EXIT HUP INT TERM
clustercontract_go=${CLUSTERCONTRACT_GO:-go}
"$clustercontract_go" build -o "$clustercontract_demo_dir/clustercontract" ./cmd/clustercontract

expect_status() {
  clustercontract_expected=$1
  shift
  if "$@"; then clustercontract_actual=0; else clustercontract_actual=$?; fi
  if [ "$clustercontract_actual" -ne "$clustercontract_expected" ]; then
    echo "Unexpected exit: got $clustercontract_actual; expected $clustercontract_expected" >&2
    exit 1
  fi
  echo "Verified CLI exit $clustercontract_actual."
}

echo 'SYNTHETIC NOT GPU VALIDATION. Importing one communicator.'
"$clustercontract_demo_dir/clustercontract" import-nccl \
  --input examples/nccl-synthetic.log --scope communicator/rack-a \
  --environment synthetic-env-v1 --recorded-at 2026-10-04T11:45:00Z \
  --reference examples/nccl-synthetic.log \
  --description 'SYNTHETIC NOT GPU VALIDATION' > "$clustercontract_demo_dir/evidence.json"

echo 'Complete communicator contract: pass.'
expect_status 0 "$clustercontract_demo_dir/clustercontract" evaluate \
  --contract examples/nccl-contract.json --evidence "$clustercontract_demo_dir/evidence.json" \
  --at 2026-10-04T12:00:00Z

echo 'Customer handover: rack-b and node storage remain incomplete despite healthy rack-a bandwidth.'
expect_status 1 "$clustercontract_demo_dir/clustercontract" evaluate \
  --contract examples/cross-tool-contract.json --evidence "$clustercontract_demo_dir/evidence.json" \
  --at 2026-10-04T12:00:00Z

echo 'High bandwidth with a correctness error: fail.'
"$clustercontract_demo_dir/clustercontract" import-nccl \
  --input examples/nccl-correctness-failure.log --scope communicator/rack-a \
  --environment synthetic-env-v1 --recorded-at 2026-10-04T11:45:00Z \
  --reference examples/nccl-correctness-failure.log \
  --description 'SYNTHETIC NOT GPU VALIDATION' > "$clustercontract_demo_dir/error-evidence.json"
expect_status 1 "$clustercontract_demo_dir/clustercontract" evaluate \
  --contract examples/nccl-contract.json --evidence "$clustercontract_demo_dir/error-evidence.json" \
  --at 2026-10-04T12:00:00Z

echo 'An explicit scoped waiver: conditional, not successful exit.'
expect_status 3 "$clustercontract_demo_dir/clustercontract" evaluate \
  --contract examples/waiver-contract.json --evidence examples/incomplete.json \
  --at 2026-10-04T12:00:00Z
