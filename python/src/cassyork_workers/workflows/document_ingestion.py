"""Temporal workflow — orchestrates Python activities (matches Go workflow name)."""

from __future__ import annotations

from datetime import timedelta
from typing import Any

from temporalio import workflow

with workflow.unsafe.imports_passed_through():
    from cassyork_workers.activities import document_pipeline as activities
    from cassyork_workers.contracts import IngestionWorkflowInput


@workflow.defn(name="DocumentIngestionWorkflow")
class DocumentIngestionWorkflow:
    @workflow.run
    async def run(self, payload: Any) -> dict[str, object]:
        data = IngestionWorkflowInput.model_validate(payload)
        await workflow.execute_activity(
            activities.validate_document_activity,
            data,
            start_to_close_timeout=timedelta(seconds=30),
        )
        extracted = await workflow.execute_activity(
            activities.extract_pages_activity,
            data,
            start_to_close_timeout=timedelta(minutes=2),
        )
        model_result = await workflow.execute_activity(
            activities.run_model_extraction_activity,
            data,
            start_to_close_timeout=timedelta(minutes=10),
        )
        normalized = await workflow.execute_activity(
            activities.normalize_output_activity,
            model_result,
            start_to_close_timeout=timedelta(seconds=30),
        )
        validation = await workflow.execute_activity(
            activities.validate_schema_activity,
            args=[data, normalized],
            start_to_close_timeout=timedelta(seconds=30),
        )
        confidence = await workflow.execute_activity(
            activities.score_confidence_activity,
            normalized,
            start_to_close_timeout=timedelta(seconds=30),
        )
        await workflow.execute_activity(
            activities.evaluate_ground_truth_activity,
            args=[data, normalized],
            start_to_close_timeout=timedelta(seconds=60),
        )
        bundle = {
            "extracted": extracted,
            "model_result": model_result,
            "normalized": normalized,
            "validation": validation,
            "field_confidence": confidence,
        }
        await workflow.execute_activity(
            activities.persist_output_activity,
            args=[data, bundle],
            start_to_close_timeout=timedelta(seconds=60),
        )
        await workflow.execute_activity(
            activities.publish_webhook_activity,
            data,
            start_to_close_timeout=timedelta(seconds=60),
        )
        return {
            "status": "completed",
            "ingestion_run_id": data.ingestion_run_id,
            "validation": validation,
        }
