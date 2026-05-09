"""Provider context — ports (implementations live in infrastructure)."""

from __future__ import annotations

from typing import Any, Protocol

from cassyork_workers.domain.document import Document
from cassyork_workers.domain.ingestion import Run


class ModelExtractor(Protocol):
    """Hides Gemini/OpenAI/Claude/Textract behind one surface."""

    async def extract(
        self,
        *,
        document: Document,
        run: Run,
        preview_text: str,
    ) -> dict[str, Any]:
        """Returns provider-agnostic result dict (parsed_output, usage, cost, ...)."""
        ...
