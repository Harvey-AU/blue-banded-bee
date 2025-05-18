# Simple Organisation Model

This document outlines a straightforward organisation model for Blue Banded Bee.

## Overview

The organisation model for Blue Banded Bee will be simple:
1. When a user creates an account, they also create an organisation
2. Users can invite others to their organisation
3. Everyone in an organisation shares access to all jobs, results, and billing
4. No complex roles or permissions - everyone has equal access

## Data Model

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│  Organisations  │◄────┤  Members        ├────►│     Users       │
│                 │     │                 │     │                 │
└────────┬────────┘     └─────────────────┘     └─────────────────┘
         │                                               
         │                                               
         │                                               
         ▼                                               
┌─────────────────┐                                      
│                 │                                      
│     Jobs        │                                      
│ (and all other  │                                      
│   resources)    │                                      
│                 │                                      
└─────────────────┘                                      
```

### Core Tables

#### Organisations

```sql
CREATE TABLE organisations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### Members

```sql
CREATE TABLE members (
    organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organisation_id, user_id)
);
```

#### Jobs
```sql
-- Update jobs table to include organisation_id
ALTER TABLE jobs ADD COLUMN organisation_id UUID NOT NULL REFERENCES organisations(id) ON DELETE CASCADE;
```

### Row Level Security

Simple Row Level Security (RLS) policies to ensure users only see their organisation's data:

```sql
-- Allow users to see resources in their organisations
CREATE POLICY "Users can access their organisation's data"
ON organisations
FOR ALL
USING (
    EXISTS (
        SELECT 1 FROM members 
        WHERE members.organisation_id = organisations.id 
        AND members.user_id = auth.uid()
    )
);

-- Same policy pattern applies to jobs and other resources
CREATE POLICY "Users can access their organisation's jobs"
ON jobs
FOR ALL
USING (
    EXISTS (
        SELECT 1 FROM members 
        WHERE members.organisation_id = jobs.organisation_id 
        AND members.user_id = auth.uid()
    )
);
```

## Implementation Steps

1. **Database Changes**:
   - Add organisation and members tables
   - Add organisation_id to jobs and other resource tables
   - Set up RLS policies

2. **User Flow**:
   - When user signs up, automatically create an organisation
   - Allow users to invite others via email
   - Share all resources within organisation

3. **Billing**:
   - Billing attached to organisation, not individual users
   - Usage quotas tracked at organisation level

## Simple API Endpoints

```
/api/organisations/members          # Add/remove members
/api/organisations/invitations      # Send/manage invitations
```