"""IngestionRun aggregate — one processing attempt (replayable, comparable)."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import StrEnum
from typing import Any


class RunStatus(StrEnum):
    QUEUED = "queued"
    RUNNING = "running"
    WAITING_ON_PROVIDER = "waiting_on_provider"
    VALIDATING = "validating"
    REQUIRES_REVIEW = "requires_review"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


@dataclass
class Run:
    id: str
    organization_id: str
    project_id: str
    document_id: str
    pipeline_id: str
    schema_id: str
    model_config_id: str
    prompt_version_id: str | None
    trace_id: str
    status: RunStatus = RunStatus.QUEUED
    started_at: datetime | None = None
    completed_at: datetime | None = None
    failed_at: datetime | None = None
    error_message: str | None = field(default=None, repr=False)

    @classmethod
    def queue(
        cls,
        *,
        id: str,
        organization_id: str,
        project_id: str,
        document_id: str,
        pipeline_id: str,
        schema_id: str,
        model_config_id: str,
        prompt_version_id: str | None,
        trace_id: str,
    ) -> Run:
        if not id or not organization_id or not project_id or not document_id or not trace_id:
            raise ValueError("run id, org, project, document_id, trace_id required")
        return cls(
            id=id,
            organization_id=organization_id,
            project_id=project_id,
            document_id=document_id,
            pipeline_id=pipeline_id,
            schema_id=schema_id,
            model_config_id=model_config_id,
            prompt_version_id=prompt_version_id,
            trace_id=trace_id,
        )

    @classmethod
    def from_db(cls, row: Mapping[str, Any]) -> Run:
        pv = row.get("prompt_version_id")
        prompt_version_id = None if pv is None or pv == "" else str(pv)
        err = row.get("error_message")
        return cls(
            id=row["id"],
            organization_id=row["organization_id"],
            project_id=row["project_id"],
            document_id=row["document_id"],
            pipeline_id=row["pipeline_id"] or "",
            schema_id=row["schema_id"] or "",
            model_config_id=row["model_config_id"] or "",
            prompt_version_id=prompt_version_id,
            trace_id=row["trace_id"],
            status=RunStatus(row["status"]),
            started_at=row["started_at"],
            completed_at=row["completed_at"],
            failed_at=row["failed_at"],
            error_message=str(err) if err is not None else None,
        )

    def mark_started(self, now: datetime | None = None) -> None:
        self._ensure_not_completed()
        if self.status != RunStatus.QUEUED:
            raise ValueError("can only start from queued")
        self.status = RunStatus.RUNNING
        ts = _utc(now)
        self.started_at = ts

    def mark_waiting_on_provider(self, now: datetime | None = None) -> None:
        self._ensure_not_completed()
        if self.status != RunStatus.RUNNING:
            raise ValueError("expected running")
        self.status = RunStatus.WAITING_ON_PROVIDER
        _ = _utc(now)

    def mark_validating(self, now: datetime | None = None) -> None:
        self._ensure_not_completed()
        self.status = RunStatus.VALIDATING
        _ = _utc(now)

    def mark_requires_review(self, now: datetime | None = None) -> None:
        self._ensure_not_completed()
        self.status = RunStatus.REQUIRES_REVIEW
        _ = _utc(now)

    def mark_completed(self, now: datetime | None = None) -> None:
        self._ensure_not_completed()
        if self.status not in (
            RunStatus.RUNNING,
            RunStatus.WAITING_ON_PROVIDER,
            RunStatus.VALIDATING,
            RunStatus.REQUIRES_REVIEW,
        ):
            raise ValueError("invalid transition to completed")
        self.status = RunStatus.COMPLETED
        self.completed_at = _utc(now)

    def mark_failed(self, message: str, now: datetime | None = None) -> None:
        self._ensure_not_completed()
        if self.status == RunStatus.COMPLETED:
            raise ValueError("immutable")
        self.status = RunStatus.FAILED
        self.failed_at = _utc(now)
        self.error_message = message

    def _ensure_not_completed(self) -> None:
        if self.status == RunStatus.COMPLETED:
            raise ValueError("completed run is immutable")


def _utc(now: datetime | None) -> datetime:
    t = now or datetime.now(UTC)
    return t if t.tzinfo else t.replace(tzinfo=UTC)
