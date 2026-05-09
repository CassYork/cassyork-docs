"""Schema/output context — pure validation rules."""

from __future__ import annotations

from typing import Any

from jsonschema import Draft202012Validator


def validate_json(data: dict[str, Any], schema: dict[str, Any]) -> list[str]:
    validator = Draft202012Validator(schema)
    return [f"{list(e.path)}: {e.message}" for e in validator.iter_errors(data)]
