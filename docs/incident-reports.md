# Incident reports

## INC-2117 — orders from other regions returned to a clerk

**Severity:** high (cross-tenant read)
**Detected:** 2025-08-14, by a customer support ticket
**Closed:** 2025-08-19

### Summary

A support clerk in the `eu` region could list orders belonging to the `us`
region whenever the clerk's own user id appeared as an order owner. Orders
owned by the clerk were returned regardless of region or order status.

### Timeline

| Time (UTC) | Event |
|---|---|
| 08-14 09:12 | Ticket: clerk reports seeing an order with a US billing address |
| 08-14 09:40 | Orders listing reproduced against staging with the clerk's subject |
| 08-14 11:05 | Production read path for `orders` frozen; reports served from a snapshot |
| 08-14 16:20 | Root cause identified in the generated predicate for the `orders` read policy |
| 08-15 10:00 | Predicate grouping contract written and reviewed |
| 08-19 14:30 | Read path restored with the corrected compiler and an expanded regression set |

### What the policy intended

The `orders` read policy has two rules that apply to a clerk:

1. orders in the subject's region that are not void;
2. orders the subject owns.

A clerk should see the union of those two sets. The first rule carries two
conditions; the second carries one.

### What the generated statement expressed

The compiled predicate placed the three comparisons in a single `WHERE`
clause without grouping the first rule:

```
WHERE "orders"."region" = $1 AND "orders"."status" <> $2 OR "orders"."owner_id" = $3
```

SQL evaluates `AND` before `OR`, so this reads as

```
(region = $1 AND status <> $2) OR owner_id = $3
```

The owner comparison escaped the region and status conditions. Any row owned
by the clerk matched, including rows in other regions and void rows.

### Analysis

The condition as written in the policy document had the intended meaning;
the translation to SQL lost it. Combining rule predicates with `OR` is only
safe when each operand carries its own grouping. The same failure applies to
`NOT`: a negated compound condition loses its boundary the moment the
parentheses are dropped.

The generated text was also hard to review. Nothing in the output distinguished
the grouping that the source expression had from the grouping SQL would infer,
so a reviewer reading the statement had to reconstruct precedence by hand.

### Follow-up

* The grouping contract is now stated in the design notes (§3).
* Read policies that combine several rules are exercised with subjects that
  match more than one rule, not only with subjects that match exactly one.
* Statement review checks the meaning of the rewritten `WHERE` clause, not
  just the presence of the expected column names.
* The staging corpus includes subjects whose ownership overlaps another rule,
  so a predicate that lets ownership escape the other conditions is visible
  before release.
