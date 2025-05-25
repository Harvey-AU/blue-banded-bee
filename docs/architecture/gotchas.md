# Known Gotchas and Edge Cases

## Database Schema Migrations

### Depth Column Legacy Issue (Fixed in v0.3.8)

**Problem**: When removing deprecated functionality from code, ensure production database schema is also updated.

**What happened**: 
- v0.3.7 removed `depth` functionality from code including INSERT statements
- Production database still contained the `depth` column with NOT NULL constraint
- Task insertion failed with `pq: null value in column "depth" of relation "tasks" violates not-null constraint`

**Root cause**: `CREATE TABLE IF NOT EXISTS` statements don't modify existing table structure

**Solution**: 
- Use database reset endpoint to recreate schema from scratch, OR
- Add explicit migration commands like `ALTER TABLE tasks DROP COLUMN IF EXISTS depth`

**Prevention**: When removing table columns from code, also plan database migration strategy for production

## Function Signature Updates

### Test Utility Compilation Issues

**Problem**: When refactoring core functions, update all dependent code including test utilities

**What happened**:
- `NewWorkerPool` and `NewJobManager` signatures changed to require `dbQueue` parameter
- `cmd/test_jobs/main.go` wasn't updated, causing compilation failures

**Solution**: Follow same patterns as main application code when updating test utilities

**Prevention**: Always check all callers of modified functions, including test utilities and examples