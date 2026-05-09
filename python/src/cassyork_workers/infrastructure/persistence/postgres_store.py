"""Postgres persistence for Document + IngestionRun (shared with Go schema)."""

from __future__ import annotations

import os
import asyncpg

from cassyork_workers.contracts import IngestionWorkflowInput
from cassyork_workers.domain.document import Document
from cassyork_workers.domain.ingestion import Run

_pool: asyncpg.Pool | None = None


def _dsn() -> str:
    return (
        os.getenv("DATABASE_URL", "").strip()
        or os.getenv("POSTGRES_URL", "").strip()
        or "postgres://temporal:temporal@127.0.0.1:5432/cassyork?sslmode=disable"
    )


async def connect() -> None:
    global _pool
    if _pool is None:
        _pool = await asyncpg.create_pool(_dsn(), min_size=1, max_size=10)


async def disconnect() -> None:
    global _pool
    if _pool is not None:
        await _pool.close()
        _pool = None


def pool() -> asyncpg.Pool:
    if _pool is None:
        raise RuntimeError("postgres not connected; call connect() from worker_main")
    return _pool


async def load_document_and_run(payload: IngestionWorkflowInput) -> tuple[Document, Run]:
    """Load authoritative aggregates by id (Temporal payload only carries correlation ids)."""
    p = pool()
    async with p.acquire() as conn:
        doc_row = await conn.fetchrow(
            """
            SELECT id, organization_id, project_id, storage_uri, mime_type,
                   checksum_sha256, page_count, status, created_at
            FROM documents WHERE id = $1
            """,
            payload.document_id,
        )
        run_row = await conn.fetchrow(
            """
            SELECT id, organization_id, project_id, document_id,
                   pipeline_id, schema_id, model_config_id, prompt_version_id,
                   trace_id, status, started_at, completed_at, failed_at, error_message
            FROM ingestion_runs WHERE id = $1
            """,
            payload.ingestion_run_id,
        )
    if doc_row is None or run_row is None:
        raise LookupError("document or ingestion_run not found — ensure ingestion-api persisted before workflow runs")
    if run_row["document_id"] != payload.document_id:
        raise ValueError("run document_id does not match payload")
    if doc_row["organization_id"] != payload.organization_id:
        raise ValueError("document organization_id mismatch")
    if run_row["organization_id"] != payload.organization_id:
        raise ValueError("run organization_id mismatch")
    if run_row["project_id"] != payload.project_id:
        raise ValueError("run project_id mismatch")
    doc = Document.from_db(doc_row)
    run = Run.from_db(run_row)
    return doc, run


async def save_run(run: Run) -> None:
    p = pool()
    err: str | None = run.error_message
    async with p.acquire() as conn:
        await conn.execute(
            """
            UPDATE ingestion_runs SET
              status = $2,
              started_at = $3,
              completed_at = $4,
              failed_at = $5,
              error_message = $6
            WHERE id = $1
            """,
            run.id,
            run.status.value,
            run.started_at,
            run.completed_at,
            run.failed_at,
            err,
        )


async def save_document(doc: Document) -> None:
    p = pool()
    checksum = doc.checksum.value if doc.checksum else None
    async with p.acquire() as conn:
        await conn.execute(
            """
            UPDATE documents SET
              page_count = $2,
              status = $3,
              checksum_sha256 = COALESCE($4, checksum_sha256)
            WHERE id = $1
            """,
            doc.id,
            doc.page_count,
            doc.lifecycle.value,
            checksum,
        )
