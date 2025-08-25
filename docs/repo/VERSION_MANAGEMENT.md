# Automated Version Bumping

This project uses the `action-bumpr` GitHub Action for automated version management.

## How it works

1. **Create a Pull Request** with your changes
2. **Add a version label** to the PR:
   - `bump:patch` - Bug fixes (1.0.0 → 1.0.1)
   - `bump:minor` - New features (1.0.0 → 1.1.0)
   - `bump:major` - Breaking changes (1.0.0 → 2.0.0)
3. **Merge the PR** - A new Git tag will be automatically created

## Default behavior

If no bump label is attached, it defaults to `patch` version bump.

## Badge

[![action-bumpr supported](https://img.shields.io/badge/bumpr-supported-ff69b4?logo=github&link=https://github.com/haya14busa/action-bumpr)](https://github.com/haya14busa/action-bumpr)
