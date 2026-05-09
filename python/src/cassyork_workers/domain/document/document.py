"""Document aggregate — uploaded artifact metadata only (no model/prompt/output)."""

from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from datetime import UTC, datetime
from enum import StrEnum
from typing import Any


class DocumentLifecycle(StrEnum):
    UPLOADED = "uploaded"
    STORED = "stored"


@dataclass(frozen=True)
class ArtifactChecksum:
    """Value object — SHA-256 hex."""

    value: str

    def __post_init__(self) -> None:
        if self.value and len(self.value) != 64:
            raise ValueError("checksum must be 64 hex chars or empty")


@dataclass
class Document:
    """Aggregate root: one stored file within a project."""

    id: str
    organization_id: str
    project_id: str
    storage_uri: str
    mime_type: str
    checksum: ArtifactChecksum | None
    page_count: int
    lifecycle: DocumentLifecycle
    created_at: datetime

    @classmethod
    def register(
        cls,
        *,
        id: str,
        organization_id: str,
        project_id: str,
        storage_uri: str,
        mime_type: str,
        checksum_hex: str,
        created_at: datetime | None = None,
    ) -> Document:
        now = created_at or datetime.now(UTC)
        cs = ArtifactChecksum(checksum_hex) if checksum_hex else None
        lifecycle = DocumentLifecycle.STORED if cs else DocumentLifecycle.UPLOADED
        mt = mime_type or "application/octet-stream"
        if not id or not organization_id or not project_id or not storage_uri:
            raise ValueError("document id, org, project, and storage_uri required")
        return cls(
            id=id,
            organization_id=organization_id,
            project_id=project_id,
            storage_uri=storage_uri,
            mime_type=mt,
            checksum=cs,
            page_count=0,
            lifecycle=lifecycle,
            created_at=now if now.tzinfo else now.replace(tzinfo=UTC),
        )

    def record_pages(self, page_count: int) -> None:
        if page_count > 0:
            self.page_count = page_count
        self.lifecycle = DocumentLifecycle.STORED

    @classmethod
    def from_db(cls, row: Mapping[str, Any]) -> Document:
        cs_raw = row.get("checksum_sha256")
        checksum = ArtifactChecksum(str(cs_raw)) if cs_raw else None
        created = row["created_at"]
        if created.tzinfo is None:
            created = created.replace(tzinfo=UTC)
        return cls(
            id=row["id"],
            organization_id=row["organization_id"],
            project_id=row["project_id"],
            storage_uri=row["storage_uri"],
            mime_type=row["mime_type"],
            checksum=checksum,
            page_count=int(row["page_count"] or 0),
            lifecycle=DocumentLifecycle(row["status"]),
            created_at=created,
        )
