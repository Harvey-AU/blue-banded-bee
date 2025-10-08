-- Create job_share_links table for public share tokens
create table if not exists job_share_links (
    id uuid primary key default gen_random_uuid(),
    job_id uuid not null references jobs(id) on delete cascade,
    token text not null unique,
    created_by uuid references users(id),
    created_at timestamptz not null default now(),
    expires_at timestamptz,
    revoked_at timestamptz,
    constraint job_share_links_token_length check (char_length(token) >= 16)
);

create index if not exists job_share_links_job_id_idx on job_share_links(job_id);
create index if not exists job_share_links_token_idx on job_share_links(token);
create index if not exists job_share_links_valid_idx on job_share_links(token) where revoked_at is null;

