#!/usr/bin/env bash
#
# Publish npm shim packages for notebooklm.
#
# Usage:
#   ./publish.sh <version> [--dry-run]
#
# Prerequisites:
#   - Built Go binaries in the artifacts/ directory (from CI or manual build):
#       artifacts/notebooklm-darwin-universal
#       artifacts/notebooklm-linux-amd64
#       artifacts/notebooklm-linux-arm64
#       artifacts/notebooklm-windows-amd64.exe
#       artifacts/notebooklm-windows-arm64.exe
#   - npm login (must be authenticated to publish)

set -euo pipefail

VERSION="${1:?Usage: publish.sh <version> [--dry-run]}"
DRY_RUN=""
if [[ "${2:-}" == "--dry-run" ]]; then
  DRY_RUN="--dry-run"
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ARTIFACTS_DIR="${SCRIPT_DIR}/artifacts"

# Verify all required binaries exist
REQUIRED_BINARIES=(
  "notebooklm-darwin-universal"
  "notebooklm-linux-amd64"
  "notebooklm-linux-arm64"
  "notebooklm-windows-amd64.exe"
  "notebooklm-windows-arm64.exe"
)

echo "==> Checking artifacts in ${ARTIFACTS_DIR}..."
for bin in "${REQUIRED_BINARIES[@]}"; do
  if [[ ! -f "${ARTIFACTS_DIR}/${bin}" ]]; then
    echo "ERROR: Missing artifact: ${ARTIFACTS_DIR}/${bin}"
    exit 1
  fi
done
echo "    All artifacts found."

# Map: platform-dir -> artifact-name -> binary-name-in-package
declare -A PLATFORM_MAP
PLATFORM_MAP["darwin-universal"]="notebooklm-darwin-universal:notebooklm"
PLATFORM_MAP["linux-x64"]="notebooklm-linux-amd64:notebooklm"
PLATFORM_MAP["linux-arm64"]="notebooklm-linux-arm64:notebooklm"
PLATFORM_MAP["win32-x64"]="notebooklm-windows-amd64.exe:notebooklm.exe"
PLATFORM_MAP["win32-arm64"]="notebooklm-windows-arm64.exe:notebooklm.exe"

# Publish platform packages
for platform in "${!PLATFORM_MAP[@]}"; do
  IFS=':' read -r artifact_name bin_name <<< "${PLATFORM_MAP[$platform]}"
  pkg_dir="${SCRIPT_DIR}/platforms/${platform}"

  echo ""
  echo "==> Publishing @missdeer/notebooklm-${platform}@${VERSION}..."

  # Update version in package.json
  cd "${pkg_dir}"
  npm version "${VERSION}" --no-git-tag-version --allow-same-version

  # Copy binary
  mkdir -p bin
  cp "${ARTIFACTS_DIR}/${artifact_name}" "bin/${bin_name}"
  chmod +x "bin/${bin_name}"

  # Publish
  npm publish ${DRY_RUN}

  # Clean up binary (don't commit it)
  rm -rf bin

  echo "    Done: @missdeer/notebooklm-${platform}@${VERSION}"
done

# Publish main wrapper package
echo ""
echo "==> Publishing @missdeer/notebooklm@${VERSION}..."
cd "${SCRIPT_DIR}/notebooklm"

# Update version and optionalDependencies versions
npm version "${VERSION}" --no-git-tag-version --allow-same-version

# Update optionalDependencies to match
node -e "
const fs = require('fs');
const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
for (const dep of Object.keys(pkg.optionalDependencies || {})) {
  pkg.optionalDependencies[dep] = '${VERSION}';
}
fs.writeFileSync('package.json', JSON.stringify(pkg, null, 2) + '\n');
"

npm publish ${DRY_RUN}

echo ""
echo "==> All packages published at version ${VERSION}!"
