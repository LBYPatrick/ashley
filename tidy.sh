#!/bin/bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
gofmt -w assets.go cmd internal scripts/release tests/integration
bash -n tidy.sh
while IFS= read -r -d '' script; do
    bash -n "$script"
done < <(find scripts -type f -name '*.sh' -print0)
echo "Go formatting and shell syntax checks complete."
