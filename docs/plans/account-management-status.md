# Account Management Status

| Page     | Section                      | UI      | Function | Notes                                                     |
| -------- | ---------------------------- | ------- | -------- | --------------------------------------------------------- |
| Settings | Account > Profile            | Done    | Partial  | Read-only name/email display; no edit/update flow.        |
| Settings | Account > Security           | Done    | Done     | Password reset email via Supabase.                        |
| Settings | Team > Members               | Done    | Done     | List + remove with admin gating.                          |
| Settings | Team > Invites               | Done    | Done     | Send/list/revoke invites; admin-only.                     |
| Settings | Plans > Current              | Done    | Done     | Uses `/v1/usage`.                                         |
| Settings | Plans > Change plan          | Done    | Done     | Uses `/v1/plans` + `/v1/organisations/plan` (admin-only). |
| Settings | Plans > Usage history        | Done    | Done     | Uses `/v1/usage/history?days=30`.                         |
| Settings | Billing > Payment methods    | Partial | Not done | Static card + disabled action; needs tabs.                |
| Settings | Billing > Invoices           | Partial | Not done | Static placeholder; needs tabs.                           |
| Settings | Notifications > Slack        | Done    | Done     | Connect/list/disconnect.                                  |
| Settings | Analytics > Google Analytics | Done    | Done     | Connect/select property/save.                             |
| Settings | Auto-crawl > Webflow         | Done    | Done     | Connect/list sites/schedule/run on publish.               |
