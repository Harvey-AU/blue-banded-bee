# Git Branching and PR Workflow

## Branch Structure

Blue Banded Bee uses a three-tier branching strategy:

```
main (production)
  └── test-branch (staging/preview)
       └── feature/your-feature (development)
```

### Branch Purposes

- **main**: Production branch, deployed to live environment
- **test-branch**: Staging branch for integration testing
- **feature branches**: Individual development work

## Development Workflow

### 1. Start New Work

```bash
# Always branch from main for new features
git checkout main
git pull origin main
git checkout -b feature/descriptive-name

# For bug fixes
git checkout -b bug/issue-description

# For documentation
git checkout -b docs/what-you-are-documenting
```

### 2. Development Process

```bash
# Make changes
# Run tests locally
./run-tests.sh

# Commit with conventional commits
git add .
git commit -m "feat: add new feature"
# or
git commit -m "fix: resolve issue with X"
```

### 3. Push Feature Branch

```bash
git push origin feature/your-feature
```

### 4. Create Pull Request

1. **First PR: feature → test-branch**
   - Tests run automatically via GitHub Actions
   - Supabase preview deploys with migrations
   - Review and test in staging environment

2. **After Testing: test-branch → main**
   - Create second PR for production
   - Final review before deployment
   - Migrations apply automatically on merge

## Commit Message Convention

Follow conventional commits for clear history:

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation only
- `style:` - Code style changes (formatting)
- `refactor:` - Code restructuring
- `test:` - Test additions/changes
- `chore:` - Maintenance tasks

Keep messages concise (5-6 words max).

## PR Guidelines

### Creating a PR

1. **Title**: Clear, descriptive summary
2. **Description**: 
   - What changed and why
   - Related issue numbers
   - Testing performed
3. **Checklist**:
   - [ ] Tests pass locally
   - [ ] Documentation updated
   - [ ] No secrets in code

### PR Review Process

1. **Automated Checks**:
   - GitHub Actions tests must pass
   - No merge conflicts
   - Supabase preview successful

2. **Review Focus**:
   - Code quality and standards
   - Test coverage
   - Documentation completeness
   - Security considerations

## Database Migrations

When your PR includes database changes:

1. **Create migration file**:
   ```bash
   supabase migration new your_migration_name
   ```

2. **Test locally**:
   ```bash
   supabase db reset
   ```

3. **Push with feature branch**:
   - Migrations auto-apply to test-branch preview
   - Review schema changes in Supabase dashboard

## Merge Strategy

- **Feature → test-branch**: Squash and merge (clean history)
- **test-branch → main**: Create merge commit (preserve context)

## Emergency Fixes

For critical production issues:

```bash
# Create hotfix from main
git checkout main
git checkout -b hotfix/critical-issue

# Fix and test
# Create PR directly to main
# Document why bypassing test-branch
```

## Branch Cleanup

After merging:

```bash
# Delete local feature branch
git branch -d feature/your-feature

# Delete remote feature branch
git push origin --delete feature/your-feature

# Prune remote branches
git remote prune origin
```

## Common Scenarios

### Updating Feature Branch

```bash
# If main has new changes
git checkout main
git pull origin main
git checkout feature/your-feature
git rebase main
```

### Multiple Developers

- Communicate about overlapping work
- Use draft PRs for work-in-progress
- Resolve conflicts early

### Long-Running Features

- Rebase regularly from main
- Consider feature flags for gradual rollout
- Break into smaller PRs when possible

## CI/CD Integration

The branching workflow integrates with:

1. **GitHub Actions**: Tests run on PR creation
2. **Supabase Previews**: Database changes deploy to test branches
3. **Fly.io Deployment**: Main branch auto-deploys

See [CI/CD Documentation](./testing/ci-cd.md) for details.