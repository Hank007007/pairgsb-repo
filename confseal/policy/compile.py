"""Compiling a loaded document into the policy object."""

from collections.abc import Mapping, Sequence
from dataclasses import dataclass

from .errors import PolicyError

MERGE_KEY = "<<"
ACTIONS = ("allow", "deny")
RULE_FIELDS = ("id", "subject", "action", "destinations", "schedule")


@dataclass(frozen=True)
class Rule:
    """One compiled rule, ready for the filter engine."""

    id: str
    subject: str
    action: str
    destinations: tuple
    schedule: Mapping
    extra: Mapping

    def to_dict(self):
        return {
            "id": self.id,
            "subject": self.subject,
            "action": self.action,
            "destinations": list(self.destinations),
            "schedule": dict(self.schedule),
            "extra": dict(self.extra),
        }


@dataclass(frozen=True)
class Policy:
    """The compiled form of one uploaded document."""

    version: int
    rules: tuple

    def to_dict(self):
        return {
            "version": self.version,
            "rules": [rule.to_dict() for rule in self.rules],
        }


def compile_policy(document):
    """Turn a loaded document into a :class:`Policy`.

    ``document`` is either the tree returned by
    :func:`confseal.intake.load_document` or the text it was read from.
    """
    if isinstance(document, str):
        from ..intake.loader import load_document

        document = load_document(document)
    if not isinstance(document, Mapping):
        raise PolicyError("invalid_document", "policy document must be a mapping")
    version = document.get("version", 1)
    if not isinstance(version, int) or isinstance(version, bool):
        raise PolicyError(
            "invalid_version",
            "version must be an integer",
            line=_line(document),
        )
    entries = document.get("rules")
    if isinstance(entries, (str, bytes)) or not isinstance(entries, Sequence):
        raise PolicyError(
            "invalid_rules",
            "rules must be a sequence",
            line=_line(document),
        )
    rules = []
    seen = set()
    for entry in entries:
        data = _resolve_mapping(entry, _line(entry))
        rule = _build_rule(data, entry)
        if rule.id in seen:
            raise PolicyError(
                "duplicate_rule_id",
                "rule id %r is used more than once" % (rule.id,),
                line=_line(entry),
            )
        seen.add(rule.id)
        rules.append(rule)
    rules.sort(key=lambda rule: rule.id)
    return Policy(version=version, rules=tuple(rules))


def _resolve_mapping(entry, line):
    """Flatten one mapping, applying the merge keys it carries."""
    if not isinstance(entry, Mapping):
        raise PolicyError(
            "invalid_rule",
            "each rule must be a mapping",
            line=line,
        )
    explicit = {}
    merged = {}
    for key, value in entry.items():
        if key == MERGE_KEY:
            for source in _merge_sources(value, line):
                merged.update(_resolve_mapping(source, line))
        else:
            explicit[key] = value
    data = dict(explicit)
    data.update(merged)
    return data


def _merge_sources(value, line):
    if isinstance(value, Mapping):
        return [value]
    if isinstance(value, Sequence) and not isinstance(value, (str, bytes)):
        sources = []
        for item in value:
            if not isinstance(item, Mapping):
                raise PolicyError(
                    "invalid_merge",
                    "merge sources must be mappings",
                    line=line,
                )
            sources.append(item)
        return sources
    raise PolicyError(
        "invalid_merge",
        "merge value must be a mapping or a sequence of mappings",
        line=line,
    )


def _build_rule(data, entry):
    line = _line(entry)
    rule_id = data.get("id")
    if not isinstance(rule_id, str) or not rule_id.strip():
        raise PolicyError(
            "missing_field",
            "rule needs a non-empty string id",
            line=line,
        )
    subject = data.get("subject")
    if not isinstance(subject, str) or not subject:
        raise PolicyError(
            "missing_field",
            "rule %r needs a non-empty subject" % (rule_id,),
            line=line,
        )
    action = data.get("action")
    if action not in ACTIONS:
        raise PolicyError(
            "invalid_action",
            "rule %r needs an action of allow or deny" % (rule_id,),
            line=line,
        )
    destinations = data.get("destinations")
    if isinstance(destinations, (str, bytes)) or not isinstance(destinations, Sequence):
        raise PolicyError(
            "invalid_destinations",
            "rule %r needs a destinations sequence" % (rule_id,),
            line=line,
        )
    compiled_destinations = []
    for item in destinations:
        if not isinstance(item, str) or not item:
            raise PolicyError(
                "invalid_destinations",
                "rule %r has a destination that is not a string" % (rule_id,),
                line=line,
            )
        compiled_destinations.append(item)
    if not compiled_destinations:
        raise PolicyError(
            "invalid_destinations",
            "rule %r has no destinations" % (rule_id,),
            line=line,
        )
    raw_schedule = data.get("schedule")
    if raw_schedule is None:
        raw_schedule = {}
    if not isinstance(raw_schedule, Mapping):
        raise PolicyError(
            "invalid_schedule",
            "rule %r has a schedule that is not a mapping" % (rule_id,),
            line=line,
        )
    schedule = _resolve_mapping(raw_schedule, line)
    compiled_schedule = {}
    for key, value in schedule.items():
        if key not in ("from", "until"):
            raise PolicyError(
                "invalid_schedule",
                "rule %r has an unknown schedule field %r" % (rule_id, key),
                line=line,
            )
        if not isinstance(value, str) or not value:
            raise PolicyError(
                "invalid_schedule",
                "rule %r has a schedule field that is not a string" % (rule_id,),
                line=line,
            )
        compiled_schedule[key] = value
    extra = {key: value for key, value in data.items() if key not in RULE_FIELDS}
    return Rule(
        id=rule_id,
        subject=subject,
        action=action,
        destinations=tuple(compiled_destinations),
        schedule=compiled_schedule,
        extra=extra,
    )


def _line(value):
    return getattr(value, "line", None)
