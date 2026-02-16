# IPLoop Credit System — Design Document

## Overview

The IPLoop credit system incentivizes users to share bandwidth from their devices in exchange for free VPN access and proxy credits. Credits are the universal currency: earned by sharing, spent on proxy requests.

## Core Principles

1. **Share to Earn** — Every byte shared earns credits
2. **Free VPN** — Any active device = free VPN access
3. **Multipliers** — More devices & uptime = more credits
4. **Append-only Ledger** — Full audit trail, no balance mutations without a ledger entry

## Credit Rates

| Action | Credits |
|---|---|
| Share 1 GB bandwidth | ~100 credits |
| 1 proxy request | ~1 credit |

## Multipliers

| Bonus | Condition | Multiplier |
|---|---|---|
| Multi-device | 3+ active devices | 2x |
| 24h Uptime | Device online 24h straight | 1.5x |
| Rare Geo | Traffic from rare countries | Up to 3x |

Multipliers stack multiplicatively. A user with 3 devices in a rare geo country sharing for 24h gets: `2.0 × 1.5 × 3.0 = 9x` credits.

## Tables

### `users`
Core user record. Stores current `credit_balance` (denormalized for fast reads) and `vpn_enabled` flag. API/SDK keys auto-generated.

### `devices`
Each physical device registered. Tracks platform, geo, and online status. `last_seen` updated by heartbeat.

### `bandwidth_contributions`
Time-series of bandwidth shared. Each row = one reporting interval (typically 5 min). Used for credit calculation and analytics.

### `credits_ledger`
**Append-only.** Every credit change is recorded with type, amount, reason, and resulting balance. This is the source of truth — `users.credit_balance` is a cache.

### `proxy_usage`
Each proxy request logged with exit node info, bytes transferred, and credits spent.

### `multiplier_rules` / `geo_multipliers`
Configurable multiplier rules and per-country geo tiers.

### `credit_rates`
Configurable rates table so we can adjust pricing without code changes.

## VPN Access Logic

```
vpn_enabled = user has at least 1 device with status = 'active'
```

Updated atomically whenever credits are processed or device status changes. The `user_vpn_status` view provides a real-time check.

## Key Functions

- **`credit_user()`** — Atomic credit/debit: updates balance + inserts ledger entry + refreshes VPN flag
- **`get_user_multiplier()`** — Calculates current multiplier for a user

## Data Flow

```
Device heartbeat → bandwidth_contributions row
                 → calculate credits (bytes × rate × multipliers)
                 → credit_user('earned', amount)
                 → ledger entry + balance update

Proxy request    → proxy_usage row
                 → credit_user('spent', -amount)
                 → ledger entry + balance update
```

## Scaling Notes

- `bandwidth_contributions` and `credits_ledger` will be the largest tables — partition by month on `ts` when needed
- All queries indexed on `(user_id, ts DESC)` for fast dashboard lookups
- `credit_balance` on users avoids summing the entire ledger for balance checks
- Consider TimescaleDB for bandwidth_contributions in production
