"""Workflow/activity payloads — keep aligned with Go `internal/workflows/payloads.go`."""

from __future__ import annotations

from pydantic import BaseModel, Field


class IngestionWorkflowInput(BaseModel):
    organization_id: str
    project_id: str
    document_id: str
    ingestion_run_id: str
    storage_uri: str
    mime_type: str
    pipeline_id: str = ""
    schema_id: str = ""
    model_config_id: str = ""
    prompt_version_id: str | None = None
    trace_id: str = ""
