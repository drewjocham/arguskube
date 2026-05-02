#!/bin/bash
# Clean build with codesign workaround
set -e
cd "$(dirname "$0")"
rm -rf build/bin
wails build 2>&1 | grep -v "codesign failed" || true
xattr -cr build/bin/kubewatcher.app 2>/dev/null || true
codesign --force --deep --sign - build/bin/kubewatcher.app
echo "✓ Build complete: build/bin/kubewatcher.app"
open build/bin/kubewatcher.app
