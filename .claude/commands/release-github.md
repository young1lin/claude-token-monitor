---
description: Create and push GitHub release tag with automatic version bumping
---

# GitHub Release Command

Automate version bumping and GitHub release creation for any project type.

## Usage

```
/release-github              # Patch (补丁更新): v0.1.2 → v0.1.3 (default)
/release-github patch        # Patch (补丁更新): v0.1.2 → v0.1.3
/release-github minor        # Minor (小版本更新): v0.1.2 → v0.2.0
/release-github major        # Major (大版本更新): v0.1.2 → v1.0.0
/release-github v1.2.3       # Specific version
/release-github revoke       # Revoke last release
/release-github revoke v0.1.5  # Revoke specific version
```

## Version Bump Types

| Type | Chinese | Rule | Example |
|------|---------|------|---------|
| patch | 补丁更新 | PATCH += 1 | 0.1.2 → 0.1.3 |
| minor | 小版本更新 | MINOR += 1, PATCH = 0 | 0.1.2 → 0.2.0 |
| major | 大版本更新 | MAJOR += 1, MINOR = 0, PATCH = 0 | 0.1.2 → 1.0.0 |

**Default**: patch (补丁更新)

## Prerequisites

1. Clean working directory (no uncommitted changes)
2. On main/master branch
3. GitHub CLI (`gh`) installed and authenticated
4. Remote is GitHub (`origin` points to github.com)

## Step 1: Pre-flight Checks

```bash
# Check for uncommitted changes (should be empty)
git status --porcelain

# Check current branch (should be main or master)
git branch --show-current

# Check gh CLI
gh version

# Check remote
git remote get-url origin
```

If any check fails, STOP and inform the user.

## Step 2: Detect Current Version (Priority Order)

**IMPORTANT**: Try these methods IN ORDER. Language-specific version files take priority over git tags.

### Priority 1: package.json (Node.js/TypeScript)
```bash
[ -f "package.json" ] && cat package.json | grep -m1 '"version"' | sed 's/.*"version": *"\([^"]*\)".*/\1/'
```

### Priority 2: pom.xml (Maven/Java)
```bash
[ -f "pom.xml" ] && cat pom.xml | sed -n '/<project/,/<\/project>/p' | grep -m1 '<version>' | sed 's/.*<version>\([^<]*\)<\/version>.*/\1/'
```

### Priority 3: Cargo.toml (Rust)
```bash
[ -f "Cargo.toml" ] && cat Cargo.toml | grep -m1 '^version' | sed 's/version *= *"\([^"]*\)".*/\1/'
```

### Priority 4: pyproject.toml (Python)
```bash
[ -f "pyproject.toml" ] && cat pyproject.toml | grep -m1 '^version' | sed 's/version *= *"\([^"]*\)".*/\1/'
```

### Priority 5: VERSION / version.txt (Go/General)
```bash
cat VERSION 2>/dev/null || cat version.txt 2>/dev/null
```

### Priority 6: build.gradle / build.gradle.kts (Gradle/Java)
```bash
[ -f "build.gradle" ] && cat build.gradle | grep -m1 'version\s*=' | sed "s/.*version\s*=\s*['\"]\([^'\"]*\)['\"].*/\1/"
```

### Priority 7: *.csproj (.NET)
```bash
csproj=$(ls *.csproj 2>/dev/null | head -1) && [ -n "$csproj" ] && cat "$csproj" | grep -m1 '<Version>' | sed 's/.*<Version>\([^<]*\)<\/Version>.*/\1/'
```

### Priority 8: setup.py (Python legacy)
```bash
[ -f "setup.py" ] && cat setup.py | grep -m1 'version\s*=' | sed "s/.*version\s*=\s*['\"]\([^'\"]*\)['\"].*/\1/"
```

### Priority 9: __version__.py (Python package)
```bash
version_py=$(find . -name "__version__.py" -o -name "_version.py" 2>/dev/null | head -1)
[ -n "$version_py" ] && cat "$version_py" | grep -m1 '__version__' | sed "s/.*__version__\s*=\s*['\"]\([^'\"]*\)['\"].*/\1/"
```

### Priority 10: Git tags (Fallback - LAST RESORT)
```bash
# Only use if NO language-specific version file exists
git tag -l 'v*' | sort -V | tail -1 | sed 's/^v//'
```

**Strip 'v' prefix** if present: `v0.1.2` → `0.1.2`

## Step 3: Calculate Next Version

Parse: `MAJOR.MINOR.PATCH`

| Input | Action |
|-------|--------|
| (none) / patch | PATCH += 1 |
| minor | MINOR += 1, PATCH = 0 |
| major | MAJOR += 1, MINOR = 0, PATCH = 0 |
| v1.2.3 / 1.2.3 | Use exact version |

## Step 4: Update Version Files

Update ALL detected version files:

### package.json
```bash
npm version 0.1.3 --no-git-tag-version
git add package.json package-lock.json 2>/dev/null
```

### pom.xml
```bash
mvn versions:set -DnewVersion=0.1.3 -DgenerateBackupPoms=false
git add pom.xml
```

### Cargo.toml
```bash
sed -i 's/^version *= *"[^"]*"/version = "0.1.3"/' Cargo.toml
git add Cargo.toml
```

### pyproject.toml
```bash
sed -i 's/^version *= *"[^"]*"/version = "0.1.3"/' pyproject.toml
git add pyproject.toml
```

### VERSION file
```bash
echo "0.1.3" > VERSION
git add VERSION
```

### build.gradle
```bash
sed -i "s/version\s*=\s*['\"][^'\"]*['\"]/version = '0.1.3'/" build.gradle
git add build.gradle
```

### *.csproj
```bash
sed -i 's/<Version>[^<]*<\/Version>/<Version>0.1.3<\/Version>/' *.csproj
git add *.csproj
```

### setup.py
```bash
sed -i "s/version\s*=\s*['\"][^'\"]*['\"]/version = '0.1.3'/" setup.py
git add setup.py
```

### __version__.py
```bash
find . -name "__version__.py" -o -name "_version.py" | while read f; do
  sed -i "s/__version__\s*=\s*['\"][^'\"]*['\"]/__version__ = '0.1.3'/" "$f"
  git add "$f"
done
```

## Step 5: Commit Version Bump

```bash
git commit -m "chore: bump version to v0.1.3

Co-Authored-By: Claude <noreply@anthropic.com>"
```

## Step 6: Create and Push Tag

```bash
git tag -a v0.1.3 -m "Release v0.1.3"
git push origin HEAD
git push origin v0.1.3
```

This triggers GitHub Actions if `.github/workflows/release.yml` matches `tags: ['v*']`.

## Step 7: Verify Release

```bash
git ls-remote --tags origin | grep v0.1.3
gh run list --workflow=release.yml --limit 1
sleep 30
gh release view v0.1.3
```

## Revoke Release

```bash
# Delete local tag
git tag -d v0.1.3

# Delete remote tag
git push origin --delete v0.1.3

# Delete GitHub release
gh release delete v0.1.3 --yes

# Revert version commit (optional, ask user first)
git revert HEAD --no-edit
git push origin HEAD
```

**Warning**: Destructive action. Confirm with user before proceeding.

## Error Handling

| Error | Solution |
|-------|----------|
| Uncommitted changes | Commit or stash first |
| Not on main branch | Switch to main |
| Tag already exists | Revoke or use different version |
| gh not authenticated | Run `gh auth login` |
| Push rejected | Pull latest first |
| GitHub Actions failed | Check logs: `gh run view` |

## Example Output

```
User: /release-github

1. Checking prerequisites...
   ✅ Working directory clean
   ✅ On branch 'main'
   ✅ GitHub CLI available

2. Detecting version...
   Found package.json: 0.1.2 (Node.js project)
   Current: 0.1.2

3. Next version (patch)...
   0.1.2 → 0.1.3

4. Updating files...
   ✅ package.json
   ✅ package-lock.json

5. Committing...
   ✅ chore: bump version to v0.1.3

6. Pushing tag...
   ✅ v0.1.3 pushed

7. Release: https://github.com/user/repo/releases/tag/v0.1.3
```
