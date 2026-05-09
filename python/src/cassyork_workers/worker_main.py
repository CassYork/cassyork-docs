"""Run: `python -m cassyork_workers.worker_main` with PYTHONPATH=python/src."""

from __future__ import annotations

import asyncio
import os

from temporalio.client import Client
from temporalio.worker import Worker

from cassyork_workers.activities import document_pipeline as pipeline_activities
from cassyork_workers.infrastructure.persistence import postgres_store
from cassyork_workers.observability import init_worker_tracing
from cassyork_workers.workflows.document_ingestion import DocumentIngestionWorkflow

TASK_QUEUE = "document-processing"


async def main() -> None:
    init_worker_tracing(os.getenv("OTEL_SERVICE_NAME", "cassyork-python-worker"))
    await postgres_store.connect()

    address = os.getenv("TEMPORAL_ADDRESS", "localhost:7233")
    namespace = os.getenv("TEMPORAL_NAMESPACE", "default")
    client = await Client.connect(address, namespace=namespace)

    worker = Worker(
        client,
        task_queue=TASK_QUEUE,
        workflows=[DocumentIngestionWorkflow],
        activities=[
            pipeline_activities.validate_document_activity,
            pipeline_activities.extract_pages_activity,
            pipeline_activities.run_model_extraction_activity,
            pipeline_activities.normalize_output_activity,
            pipeline_activities.validate_schema_activity,
            pipeline_activities.score_confidence_activity,
            pipeline_activities.evaluate_ground_truth_activity,
            pipeline_activities.persist_output_activity,
            pipeline_activities.publish_webhook_activity,
        ],
    )
    try:
        await worker.run()
    finally:
        await postgres_store.disconnect()


if __name__ == "__main__":
    asyncio.run(main())
