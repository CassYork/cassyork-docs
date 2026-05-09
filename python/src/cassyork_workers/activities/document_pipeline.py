"""Temporal activity adapters — thin wrappers around application services + Postgres."""

from __future__ import annotations

from typing import Any

from temporalio import activity

from cassyork_workers.application.services.ingestion_pipeline import IngestionPipelineService
from cassyork_workers.contracts import IngestionWorkflowInput
from cassyork_workers.infrastructure.persistence import postgres_store
from cassyork_workers.infrastructure.providers.stub_provider import StubExtractor

_pipeline: IngestionPipelineService | None = None


def _svc() -> IngestionPipelineService:
    global _pipeline
    if _pipeline is None:
        _pipeline = IngestionPipelineService(StubExtractor())
    return _pipeline


@activity.defn
async def validate_document_activity(payload: IngestionWorkflowInput) -> dict[str, Any]:
    doc, run = await postgres_store.load_document_and_run(payload)
    out = await _svc().validate_document(doc, run)
    await postgres_store.save_run(run)
    return out


@activity.defn
async def extract_pages_activity(payload: IngestionWorkflowInput) -> dict[str, Any]:
    doc, run = await postgres_store.load_document_and_run(payload)
    out = await _svc().extract_pages(doc, run)
    await postgres_store.save_document(doc)
    await postgres_store.save_run(run)
    return out


@activity.defn
async def run_model_extraction_activity(payload: IngestionWorkflowInput) -> dict[str, Any]:
    doc, run = await postgres_store.load_document_and_run(payload)
    preview = f"[stub text for {doc.storage_uri}]"
    out = await _svc().run_model(doc, run, preview)
    await postgres_store.save_run(run)
    return out


@activity.defn
async def normalize_output_activity(model_result: dict[str, Any]) -> dict[str, Any]:
    return _svc().normalize_output(model_result)


@activity.defn
async def validate_schema_activity(
    payload: IngestionWorkflowInput, normalized: dict[str, Any]
) -> dict[str, Any]:
    _, run = await postgres_store.load_document_and_run(payload)
    return _svc().validate_schema(run, normalized, payload.schema_id)


@activity.defn
async def score_confidence_activity(normalized: dict[str, Any]) -> dict[str, Any]:
    return _svc().score_confidence(normalized)


@activity.defn
async def evaluate_ground_truth_activity(
    payload: IngestionWorkflowInput, normalized: dict[str, Any]
) -> dict[str, Any] | None:
    _, run = await postgres_store.load_document_and_run(payload)
    return await _svc().evaluate_ground_truth(run, normalized)


@activity.defn
async def persist_output_activity(
    payload: IngestionWorkflowInput, bundle: dict[str, Any]
) -> dict[str, Any]:
    _, run = await postgres_store.load_document_and_run(payload)
    out = _svc().persist_bundle(run, bundle)
    await postgres_store.save_run(run)
    return out


@activity.defn
async def publish_webhook_activity(payload: IngestionWorkflowInput) -> dict[str, Any]:
    _, run = await postgres_store.load_document_and_run(payload)
    return _svc().publish_webhook(run)
