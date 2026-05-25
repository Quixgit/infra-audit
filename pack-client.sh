#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
DIST_DIR="$PROJECT_DIR/dist"
PKG_NAME="infrajump-audit-client"
PKG_DIR="$DIST_DIR/$PKG_NAME"

rm -rf "$PKG_DIR"
mkdir -p "$PKG_DIR/bin" "$PKG_DIR/assets"

echo "=== Building client release package ==="

build_target() {
  local goos="$1"
  local goarch="$2"
  local outdir="$PKG_DIR/bin/${goos}-${goarch}"

  mkdir -p "$outdir"

  echo "Building ${goos}-${goarch}..."

  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -o "$outdir/infra-audit" ./cmd/infra-audit

  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -o "$outdir/code-audit" ./cmd/code-audit

  chmod +x "$outdir/infra-audit" "$outdir/code-audit"
}

cd "$PROJECT_DIR"

go build ./...

build_target linux amd64
build_target linux arm64
build_target darwin amd64
build_target darwin arm64

cp -R "$PROJECT_DIR/assets/." "$PKG_DIR/assets/"
cp "$PROJECT_DIR/release/install.sh" "$PKG_DIR/install.sh"
chmod +x "$PKG_DIR/install.sh"

cat > "$PKG_DIR/README.txt" <<'EOF'
InfraJump Audit Tools - Client Package

Install:
  chmod +x install.sh
  ./install.sh

After installation:
  export DO_TOKEN='dop_v1_...'
  infra-audit list-do-projects

Example:
  infra-audit all-do \
    --client 'Client Name' \
    --project 'Project Security Audit' \
    --do-project-id 'PROJECT_UUID' \
    --scope-mode hybrid \
    --out out/client-name

Tools installed:
  infra-audit
  code-audit

No source code is included in this package.
EOF

cd "$DIST_DIR"
tar -czf "$PKG_NAME.tar.gz" "$PKG_NAME"

echo ""
echo "Package created:"
echo "  $DIST_DIR/$PKG_NAME.tar.gz"
echo ""
echo "Contents:"
tar -tzf "$DIST_DIR/$PKG_NAME.tar.gz" | head -80
