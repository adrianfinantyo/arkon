# [04] Inhibition Locks & State Management

Type: grilling
Status: open
Blocked by: 01

## Question

How should `arkon inhibit` record temporary shutdown cancellations, daytime locks, and maintenance windows on the Root SSD (`/var/lib/arkon/state.json`) so that the automated nightly shutdown timer respects active locks, auto-expires stale inhibitions, and provides clear diagnostic reasons via `arkon status`?
