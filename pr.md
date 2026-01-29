# Add Label-Based Group Replica Response Strategy

## Summary

This PR enhances the `GROUP_REPLICA` partial response strategy to support label-based group and quorum identification, enabling more flexible failure tolerance for replicated data setups like `aligned_ketama` hashring.

## New Flags

- `--query.group-replica.group-label`: External label name identifying the group (stores with same value hold replicated data)
- `--query.group-replica.quorum-label`: External label name whose value specifies minimum healthy stores required per group

## How It Works

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Thanos Query                                   │
│                                                                          │
│  Flags:                                                                  │
│    --query.group-replica.group-label=receive_group                       │
│    --query.group-replica.quorum-label=quorum                             │
│    --query.partial-response.strategy=GROUP_REPLICA                       │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
            ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
            │  Receive-0   │ │  Receive-1   │ │  Receive-2   │
            │              │ │              │ │              │
            │ Labels:      │ │ Labels:      │ │ Labels:      │
            │ receive_group│ │ receive_group│ │ receive_group│
            │   ="group-A" │ │   ="group-A" │ │   ="group-A" │
            │ quorum="2"   │ │ quorum="2"   │ │ quorum="2"   │
            └──────────────┘ └──────────────┘ └──────────────┘
                    │               │               │
                    └───────────────┴───────────────┘
                                    │
                         Group "group-A" (quorum=2)
                         Needs 2 of 3 stores healthy
```

## Behavior

| Scenario | Result |
|----------|--------|
| Group has `>= quorum` healthy stores | Query succeeds for that group |
| Group has `< quorum` healthy stores | Query aborts with error |
| Store missing labels or invalid quorum | Treated as "must-success" (any failure aborts) |
| Flags not configured | Falls back to legacy DNS-based strategy |

## Example Configuration

### Receive pods with external labels:

```yaml
# receive-0 in AZ-1
- --label=receive_group="ordinal-0"
- --label=quorum="2"

# receive-1 in AZ-2 (replica of receive-0)
- --label=receive_group="ordinal-0"
- --label=quorum="2"

# receive-2 in AZ-3 (replica of receive-0)
- --label=receive_group="ordinal-0"
- --label=quorum="2"
```

### Query configuration:

```yaml
- --query.partial-response.strategy=GROUP_REPLICA
- --query.group-replica.group-label=receive_group
- --query.group-replica.quorum-label=quorum
```

### Failure scenarios:

```
Group "ordinal-0" has 3 stores, quorum=2:

  ✓ receive-0 (AZ-1) - healthy
  ✗ receive-1 (AZ-2) - failed
  ✓ receive-2 (AZ-3) - healthy

  Result: 2 >= 2 (quorum met) → Query succeeds
```

```
Group "ordinal-0" has 3 stores, quorum=2:

  ✗ receive-0 (AZ-1) - failed
  ✗ receive-1 (AZ-2) - failed
  ✓ receive-2 (AZ-3) - healthy

  Result: 1 < 2 (quorum not met) → Query aborts
```

## Label Stripping

Both `group-label` and `quorum-label` are automatically stripped from query results (similar to replica labels with deduplication).

## Backward Compatibility

- When flags are not set, the existing DNS-based `GROUP_REPLICA` behavior is preserved
- No changes required for existing deployments
