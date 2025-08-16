# Git Branching and PR Workflow

## Branch Structure

Blue Banded Bee uses a simplified branching strategy:

```
main (production) ← Direct PRs from feature branches
  └── test-branch (staging/preview) ← Only for UX testing
```

### Branch Purposes

- **main**: Production branch, deployed to live environment
- **test-branch**: Staging branch for user experience testing only
- **feature branches**: Individual development work (deleted after merge)

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

**Primary Workflow: Direct to Main**

1. **Feature → main (default)**:
   - Create PR directly to main branch
   - Tests run automatically via GitHub Actions
   - Code review and approval
   - Merge and automatically delete feature branch

**Optional: User Experience Testing**

2. **Feature → test-branch (when UX testing needed)**:
   - Only use test-branch for user experience validation
   - Supabase preview deploys with migrations
   - Test UI/UX changes in staging environment
   - After validation: merge test-branch → main

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

## Branch Cleanup Policy

**Mandatory**: All feature branches must be deleted after merging to keep the repository clean.

### Automatic Cleanup

- GitHub can auto-delete branches after PR merge (recommended setting)
- Use "Squash and merge" for feature branches to maintain clean history

### Manual Cleanup

If not automated:

```bash
# Delete local feature branch
git branch -d feature/your-feature

# Delete remote feature branch  
git push origin --delete feature/your-feature

# Prune stale remote references
git remote prune origin
```

### Branch Lifecycle

1. **Create**: Branch from main for new work
2. **Develop**: Make changes and commit
3. **PR**: Create pull request (usually to main)
4. **Review**: Code review and testing
5. **Merge**: Squash and merge to target branch
6. **Delete**: Immediately delete the feature branch

**Exception**: Only `test-branch` persists for ongoing UX testing needs.

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