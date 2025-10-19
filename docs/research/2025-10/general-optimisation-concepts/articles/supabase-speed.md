https://medium.com/@kaushalsinh73/8-supabase-postgres-habits-for-startup-speed-backends-9acbff48f0aa

8 Supabase Postgres Habits for Startup-Speed Backends Practical habits that keep
your Supabase API fast, safe, and cheap — so you can ship product, not yak-shave
infra. Neurobyte Neurobyte

Follow 5 min read · 2 days ago 2

Press enter or click to view image in full size

Eight Supabase + Postgres habits — RLS-first design, smart indexes, RPCs, JSONB
discipline, pg_cron jobs, pooling, and observability — to keep backends fast.

Let’s be real: most early backends die by a thousand “it’s fine for now”
decisions. Supabase gives you an excellent starting line — Postgres, auth,
storage, realtime — but the speed you keep comes from habits. Here are eight
that consistently pay off.

1. Treat RLS as a Product Feature (Not a Checkbox) Turn on Row Level Security
   day one and model access with policies tied to your auth. It prevents “just
   this one endpoint” leaks later.

-- Example: multitenant orgs with user membership create table org_members (
org_id uuid not null, user_id uuid not null, role text not null check (role in
('owner','admin','member')), primary key (org_id, user_id) );

create table projects ( id uuid primary key default gen_random_uuid(), org_id
uuid not null references org_members(org_id), name text not null, created_by
uuid not null );

alter table org_members enable row level security; alter table projects enable
row level security;

-- default deny create policy "org members can read org rows" on projects for
select to authenticated using (exists ( select 1 from org_members m where
m.org_id = projects.org_id and m.user_id = auth.uid() ));

-- writers only create policy "admins write" on projects for all to
authenticated using (exists ( select 1 from org_members m where m.org_id =
projects.org_id and m.user_id = auth.uid() and m.role in ('owner','admin') ))
with check (exists ( select 1 from org_members m where m.org_id =
projects.org_id and m.user_id = auth.uid() and m.role in ('owner','admin') ));
Why it works: You encode tenancy and roles once — in the database — so every
client (web, mobile, cron) inherits the same safety net.

2. Index the Way You Query (Composite, Partial, Covering) Don’t hoard
   single-column indexes. Mirror real filters and sort orders; add partial
   indexes for sparse conditions and INCLUDE columns to avoid heap lookups.

-- Frequent query: by org_id ordered by created_at DESC, -- often filtered to
active projects only alter table projects add column active boolean default
true;

create index projects_org_created_active_idx on projects (org_id, created_at
desc) where active include (name); Rule of thumb: One composite index per hot
path (not per column). Measure with EXPLAIN ANALYZE.

3. JSONB with Discipline: Generated Columns + GIN Use JSONB for flexible bits,
   but expose generated columns for things you filter on, and index both.

create table events ( id bigserial primary key, org_id uuid not null, data jsonb
not null, -- pull common filters out for speed event_type text generated always
as ((data->>'type')) stored, user_email text generated always as
(lower(data->>'email')) stored );

create index events_org_type_ts_idx on events (org_id, event_type, id desc);
create index events_data_gin on events using gin (data jsonb_path_ops); create
unique index events_email_unique on events (user_email) where event_type =
'signup'; Why it works: You keep schema agility and predictable performance.

4. Shape Hot Paths with RPCs (One Round Trip, Consistent Logic) For write flows
   or multi-step reads, wrap logic as Postgres functions exposed via Supabase
   RPC. Use SECURITY DEFINER sparingly to perform privileged work under RLS.

create or replace function create_project(p_org uuid, p_name text) returns uuid
language plpgsql security definer set search_path = public as $$ declare new_id
uuid; begin -- authorize: caller must be admin in org if not exists ( select 1
from org_members where org_id = p_org and user_id = auth.uid() and role in
('owner','admin') ) then raise exception 'forbidden'; end if;

insert into projects (org_id, name, created_by) values (p_org, p_name,
auth.uid()) returning id into new_id;

return new_id; end$$;

grant execute on function create_project(uuid, text) to authenticated;

// supabase-js await supabase.rpc('create_project', { p_org: orgId, p_name: name
}); Why it works: One call, fewer race conditions, and business rules live with
the data.

5. Views for “Joined” APIs (Goodbye N+1) Expose SQL views that join and
   pre-aggregate what the UI needs, then select them via PostgREST (the Supabase
   REST API).

create view v*project_summary as select p.id, p.name, p.org_id, count(t.*)
filter (where t.done) as done*tasks, count(t.*) filter (where not t.done) as
open_tasks, max(t.updated_at) as last_activity from projects p left join tasks t
on t.project_id = p.id group by p.id;

-- RLS applies if underlying tables have it and you add SECURITY BARRIER if
needed

const { data } = await supabase.from('v_project_summary') .select('\*')
.eq('org_id', orgId) .order('last_activity', { ascending: false }); Payoff:
Minimal round trips; your feed loads fast without a forest of client joins.

6. Background Work the Boring Way: pg_cron + Outbox Use the outbox pattern for
   reliable side effects; schedule maintenance and retries with pg_cron
   (Supabase supports the extension).

-- Outbox for webhooks / async tasks create table outbox ( id bigserial primary
key, topic text not null, payload jsonb not null, attempt int default 0,
next_run timestamptz default now() );

-- Enqueue from triggers create or replace function after_task_insert() returns
trigger language plpgsql as

$$
begin
  insert into outbox (topic, payload)
  values ('task.created', jsonb_build_object('task_id', new.id));
  return new;
end$$;

create trigger trg_task_outbox after insert on tasks for each row execute
function after_task_insert();

-- Retry job (every minute) select cron.schedule('outbox-worker', '_/1 _ \* \*
\*',


$$

select perform_outbox_batch(); -- write this function to send webhooks and
reschedule failures

$$
);
Why it works: Exactly-once within the DB, at-least-once to the world, with simple, observable retries.

7) Pooling, Timeouts, and Chatty Query Diet
Lean on Supabase’s connection pooling and set strict timeouts so mistakes don’t snowball.

-- Safety valves (per DB)
alter database postgres
  set statement_timeout = '2s',
  set idle_in_transaction_session_timeout = '5s';

-- Optional: for one noisy query pattern, create a fast path via RPC to avoid chatty client loops
Client habit: Prefer one RPC/view call over five small selects. For dashboards, batch with select=*,related(*) to let PostgREST do the join server-side.

8) Observe Like You Mean It: pg_stat_statements, EXPLAIN, Budgets
Turn on the extensions and make slow queries visible; keep a lightweight performance checklist in your repo.

-- Enable once
create extension if not exists pg_stat_statements;

-- Top offenders
select query, calls, total_exec_time, mean_exec_time
from pg_stat_statements
order by total_exec_time desc
limit 20;

-- Investigate a hot query
explain analyze buffers
select * from v_project_summary where org_id = '…' order by last_activity desc limit 20;
Team habit: Every PR that adds a query includes an EXPLAIN plan screenshot (or paste), plus the expected index. No guesswork, no “we’ll fix it later.”

Tiny migration discipline (worth the minute)
Use the Supabase CLI migrations folder; name files by intent (2025-10-05_add_project_indexes.sql), keep them idempotent, and document the expected wins: “feed query from 210 ms → 35 ms.”

Closing thoughts
Supabase accelerates you by bundling the right primitives. These habits keep you fast: RLS first, indexes that match reality, RPCs/views for hot paths, JSONB with guardrails, pg_cron outbox, pooling + timeouts, and ruthless observability. You might be wondering, “Which two should I adopt today?” Start with RLS policies and composite/partial indexes — you’ll feel the difference this week.
$$
