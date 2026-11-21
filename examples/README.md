# Examples

## `partner_template.yaml`

A deployment-unit template received from an integration partner, kept as-is
for reference.  It is roughly 6 KB and describes a chain of profiles
(`l0` … `l7`), each referencing the previous one ten times, with `shard`
pointing at the last level.

Loading it is the closest reproduction of INC-2417
(`docs/incident-reports.md`).
