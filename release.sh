#!/bin/bash
set -e

# 🌳 Get current branch
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")

if [ "$BRANCH" != "main" ] && [ "$BRANCH" != "master" ]; then
    echo "⚠️  You are on branch '$BRANCH'. Releases are usually made from 'main' or 'master'."
    read -p "Do you want to continue? (y/n) " CONT
    if [ "$CONT" != "y" ] && [ "$CONT" != "Y" ]; then
        exit 1
    fi
fi

# 📥 Fetch tags from remote to ensure we are up to date
echo "📥 Fetching tags from origin..."
git fetch --tags origin

# 📌 Get current version based on tags
CURRENT_VERSION=$(git describe --tags --always 2>/dev/null || echo "v0.0.0")
echo "📌 Current Version: $CURRENT_VERSION"

# 🚀 Prompt for the new version
read -p "🚀 Enter new version (e.g., v1.0.1): " NEW_VERSION

if [ -z "$NEW_VERSION" ]; then
  echo "❌ Version cannot be empty."
  exit 1
fi

# 🛡️ Ensure tag doesn't already exist
if git rev-parse "$NEW_VERSION" >/dev/null 2>&1; then
  echo "❌ Tag '$NEW_VERSION' already exists locally."
  exit 1
fi

# 🏷️ Create annotated tag
echo "🏷️ Creating tag '$NEW_VERSION'..."
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"

# 📤 Push tag to remote
echo "📤 Pushing tag to GitHub..."
git push origin "$NEW_VERSION"

echo "✅ Success! Tag '$NEW_VERSION' has been created and pushed."
