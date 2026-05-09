"""Application service — orchestrates domain + provider port (no Temporal/DB here)."""

from __future__ import annotations

import hashlib
from datetime import UTC, datetime
from typing import Any

from cassyork_workers.domain.document import Document
from cassyork_workers.domain.ingestion import Run, RunStatus
from cassyork_workers.domain.providers.ports import ModelExtractor
from cassyork_workers.domain.schema_output.validate import validate_json


class IngestionPipelineService:
    """Operational core: Document → Run phases → normalized output (until persistence wired)."""

    def __init__(self, extractor: ModelExtractor) -> None:
        self._extractor = extractor

    async def validate_document(self, doc: Document, run: Run) -> dict[str, Any]:
        run.mark_started()
        return {
            "document_id": doc.id,
            "ok": True,
            "checked_at": datetime.now(UTC).isoformat(),
            "run_status": run.status.value,
        }

    async def extract_pages(self, doc: Document, run: Run) -> dict[str, Any]:
        run.mark_waiting_on_provider()
        text = f"[stub text for {doc.storage_uri}]"
        digest = hashlib.sha256(text.encode()).hexdigest()
        doc.record_pages(1)
        return {
            "page_count": 1,
            "text_sha256": digest,
            "preview": text[:200],
            "run_status": run.status.value,
        }

    async def run_model(self, doc: Document, run: Run, preview: str) -> dict[str, Any]:
        result = await self._extractor.extract(document=doc, run=run, preview_text=preview)
        run.mark_validating()
        return result

    def normalize_output(self, model_result: dict[str, Any]) -> dict[str, Any]:
        return dict(model_result.get("parsed_output") or {})

    def validate_schema(self, run: Run, normalized: dict[str, Any], schema_id: str) -> dict[str, Any]:
        _ = schema_id
        schema = _demo_manifest_schema()
        errors = validate_json(normalized, schema)
        status = "passed" if not errors else "failed"
        return {
            "validation_status": status,
            "errors": errors,
            "run_status": run.status.value,
        }

    def score_confidence(self, normalized: dict[str, Any]) -> dict[str, Any]:
        return {k: 0.9 for k in normalized}

    async def evaluate_ground_truth(
        self, run: Run, normalized: dict[str, Any]
    ) -> dict[str, Any] | None:
        _ = run, normalized
        return None

    def persist_bundle(self, run: Run, bundle: dict[str, Any]) -> dict[str, Any]:
        validation = bundle.get("validation") or {}
        vs = validation.get("validation_status")
        if vs == "failed":
            if run.status != RunStatus.COMPLETED:
                run.mark_requires_review()
        elif vs == "passed":
            run.mark_completed()
        return {"persisted": True, "run_id": run.id, "run_status": run.status.value}

    def publish_webhook(self, run: Run) -> dict[str, Any]:
        return {"queued": True, "ingestion_run_id": run.id, "run_status": run.status.value}


def _demo_manifest_schema() -> dict[str, Any]:
    return {
        "type": "object",
        "properties": {
            "manifest_number": {"type": "string"},
            "generator_name": {"type": "string"},
            "destination_facility": {"type": "string"},
            "quantity": {"type": "number"},
            "unit": {"type": "string"},
        },
        "required": ["manifest_number", "generator_name", "destination_facility"],
    }
