-- Enable pg_stat_statements for query-level observability
create extension if not exists pg_stat_statements;

-- Dedicated schema for performance insights exposed to internal tooling only
create schema if not exists observability;

-- View of the busiest statements (ordered by total execution time)
create or replace view observability.pg_stat_statements_top_total_time as
select
    queryid,
    query,
    calls,
    total_exec_time,
    mean_exec_time,
    rows,
    shared_blks_hit,
    shared_blks_read,
    local_blks_hit,
    local_blks_read,
    temp_blks_written,
    blk_read_time,
    blk_write_time
from pg_stat_statements
where query is not null
order by total_exec_time desc
limit 50;

comment on view observability.pg_stat_statements_top_total_time is
    'Top 50 statements ordered by total execution time (requires service role access)';

-- Restrict exposure to the service role and postgres superuser
revoke all on schema observability from public;
grant usage on schema observability to postgres, service_role;
grant select on all tables in schema observability to postgres, service_role;

-- Ensure future views in the schema inherit the same privileges
alter default privileges in schema observability
grant select on tables to postgres, service_role;
