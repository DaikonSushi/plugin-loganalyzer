#!/bin/bash

# Plugin Log Analyzer - GitHub Release Script
# This script helps you publish the plugin to GitHub

set -e

echo "🚀 Plugin Log Analyzer - GitHub Release Script"
echo "================================================"
echo ""

# Check if version is provided
if [ -z "$1" ]; then
    echo "❌ Error: Version number required"
    echo ""
    echo "Usage: ./publish.sh <version>"
    echo "Example: ./publish.sh 1.0.0"
    echo ""
    exit 1
fi

VERSION=$1
TAG="v${VERSION}"

echo "📦 Preparing release ${TAG}"
echo ""

# Check if git repo is clean
if [ -n "$(git status --porcelain)" ]; then
    echo "⚠️  Warning: You have uncommitted changes"
    echo ""
    git status --short
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "❌ Aborted"
        exit 1
    fi
fi

# Check if tag already exists
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "❌ Error: Tag ${TAG} already exists"
    echo ""
    echo "To delete the tag:"
    echo "  git tag -d ${TAG}"
    echo "  git push origin :refs/tags/${TAG}"
    echo ""
    exit 1
fi

# Verify go.mod has replace directive (for local dev)
if ! grep -q "^replace github.com/DaikonSushi/bot-platform" go.mod; then
    echo "⚠️  Warning: go.mod doesn't have replace directive"
    echo "This is OK for GitHub release, but may fail local build"
    echo ""
fi

# Test local build
echo "🔨 Testing local build..."
go mod tidy
go build -ldflags="-s -w" -o loganalyzer-plugin-test .
rm loganalyzer-plugin-test
echo "✅ Local build successful"
echo ""

# Create and push tag
echo "🏷️  Creating tag ${TAG}..."
git tag -a "${TAG}" -m "Release ${TAG}"
echo "✅ Tag created"
echo ""

echo "📤 Pushing tag to GitHub..."
git push origin "${TAG}"
echo "✅ Tag pushed"
echo ""

echo "🎉 Release ${TAG} published!"
echo ""
echo "GitHub Actions will now:"
echo "  1. Build binaries for all platforms"
echo "  2. Create a GitHub Release"
echo "  3. Upload all binaries to the release"
echo ""
echo "Check progress at:"
echo "  https://github.com/DaikonSushi/plugin-loganalyzer/actions"
echo ""
echo "Release will be available at:"
echo "  https://github.com/DaikonSushi/plugin-loganalyzer/releases/tag/${TAG}"
echo ""
