"""Stub model adapter — swap for real provider clients without touching domain."""

from __future__ import annotations

from typing import Any

from cassyork_workers.domain.document import Document
from cassyork_workers.domain.ingestion import Run


class StubExtractor:
    async def extract(
        self,
        *,
        document: Document,
        run: Run,
        preview_text: str,
    ) -> dict[str, Any]:
        _ = document, preview_text
        parsed = {
            "manifest_number": "MNF-STUB",
            "generator_name": "Stub Corp",
            "destination_facility": "Stub Landfill",
            "quantity": 1.0,
            "unit": "tons",
        }
        return {
            "provider": "stub",
            "model": "stub-v1",
            "raw_output": {"stub": True},
            "parsed_output": parsed,
            "usage": {"input_tokens": 100, "output_tokens": 50},
            "latency_ms": 12,
            "estimated_cost": 0.001,
            "warnings": [],
        }
