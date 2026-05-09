Below is the architecture I’d build if the product is:

> A document AI ingestion control plane that standardizes extraction, compares models, hot-switches providers, validates outputs, and proves reliability against ground truth.

The key idea: **separate ingestion, orchestration, extraction, evaluation, and observability.** Do not let “call Gemini and parse JSON” become your architecture.

---

# 1. Core Architecture

```txt
Client / Customer System
        ↓
Document Ingestion API
        ↓
Object Storage
        ↓
Ingestion Run Created
        ↓
Workflow Orchestrator
        ↓
Document Processing Pipeline
        ↓
Model Provider Layer
        ↓
Normalization + Validation
        ↓
Evaluation + Scoring
        ↓
Structured Output API / Webhook / Export

```

Recommended stack:


| Layer                  | Recommendation                              |
| ---------------------- | ------------------------------------------- |
| API                    | FastAPI, Go, or NestJS                      |
| Workflow orchestration | Temporal                                    |
| Queue/event stream     | Kafka, Redpanda, or NATS JetStream          |
| Object storage         | S3                                          |
| Primary DB             | Postgres                                    |
| Search/debug store     | OpenSearch or ClickHouse later              |
| Observability          | OpenTelemetry + Grafana + Loki + Prometheus |
| AI workers             | Python                                      |
| Hot-path preprocessing | Go/Rust only if needed                      |
| Auth                   | API keys + org/project/workspace scoping    |


For MVP, I’d use:

```txt
FastAPI
Postgres
S3
Temporal
Python workers
OpenTelemetry
Grafana/Loki/Prometheus

```

Do not start with Kafka unless you need event streaming from day one. Temporal can carry the orchestration early.

---

# 2. Main Domain Objects

You need these primitives.

## Organization

```ts
Organization {
  id
  name
  plan
  created_at
}

```

## Project

A project represents a customer workspace or use case.

```ts
Project {
  id
  organization_id
  name
  default_schema_id
  default_pipeline_id
  created_at
}

```

Examples:

```txt
Milestone Manifest Extraction
DTE CCR Ticket Processing
Insurance Claims Intake
Range Waste Manifest Processing

```

---

## Document

The uploaded file itself.

```ts
Document {
  id
  organization_id
  project_id
  original_filename
  mime_type
  file_size
  storage_uri
  checksum_sha256
  page_count
  status
  created_at
}

```

Important: documents and runs are separate.

Same document can be re-run through different:

- providers
- models
- prompts
- schemas
- preprocessing strategies
- validation rules

---

## Ingestion Run

This is the heart of the system.

```ts
IngestionRun {
  id
  organization_id
  project_id
  document_id

  pipeline_id
  schema_id
  model_config_id
  prompt_version_id

  status
  started_at
  completed_at
  failed_at

  idempotency_key
  trace_id

  raw_output_uri
  normalized_output_json
  validation_status
  confidence_score
  ground_truth_score

  error_code
  error_message
}

```

This is what you sell.

Not “we extracted a PDF.”

You sell:

> Every extraction is versioned, replayable, comparable, scored, and observable.

---

## Pipeline

Defines the steps.

```ts
Pipeline {
  id
  project_id
  name
  version
  steps_json
  is_active
}

```

Example:

```json
{
  "steps": [
    "detect_document_type",
    "extract_pages",
    "run_model",
    "normalize_output",
    "validate_schema",
    "score_confidence",
    "compare_ground_truth",
    "publish_result"
  ]
}

```

---

## Model Config

Abstraction over providers.

```ts
ModelConfig {
  id
  organization_id
  provider
  model_name
  version_label
  temperature
  max_tokens
  response_format
  timeout_ms
  retry_policy
  cost_per_1k_tokens
  is_active
}

```

Examples:

```txt
gemini-2.5-pro-manifest-v4
gpt-4.1-invoice-v2
claude-sonnet-claim-v1
textract-baseline-v1
document-ai-form-parser-v1

```

---

## Prompt Version

```ts
PromptVersion {
  id
  project_id
  name
  version
  prompt_text
  system_prompt
  output_schema_id
  created_by
  created_at
}

```

This allows prompt regression testing.

You need to know:

```txt
Did prompt v17 improve destination facility extraction?
Did it hurt generator name accuracy?
Did it break date normalization?

```

---

## Extraction Schema

```ts
ExtractionSchema {
  id
  project_id
  name
  version
  json_schema
  required_fields
  field_descriptions
  created_at
}

```

Example for manifests:

```json
{
  "type": "object",
  "properties": {
    "generator_name": { "type": "string" },
    "manifest_number": { "type": "string" },
    "waste_description": { "type": "string" },
    "pickup_date": { "type": "string", "format": "date" },
    "destination_facility": { "type": "string" },
    "quantity": { "type": "number" },
    "unit": { "type": "string" }
  },
  "required": [
    "manifest_number",
    "generator_name",
    "destination_facility"
  ]
}

```

---

## Ground Truth

```ts
GroundTruth {
  id
  document_id
  schema_id
  expected_output_json
  created_by
  source
  reviewed_at
}

```

Ground truth is your moat.

This is what lets you say:

> Provider A is 94.2% accurate on this customer’s actual documents. Provider B is cheaper but fails on handwritten destination facilities.

---

## Evaluation Result

```ts
EvaluationResult {
  id
  ingestion_run_id
  document_id
  model_config_id
  prompt_version_id
  schema_id

  field_scores_json
  total_score
  exact_match_score
  semantic_match_score
  missing_required_fields
  hallucinated_fields
  validation_errors

  created_at
}

```

Example field score:

```json
{
  "manifest_number": {
    "expected": "MNF-102991",
    "actual": "MNF-102991",
    "score": 1.0,
    "match_type": "exact"
  },
  "destination_facility": {
    "expected": "Republic Imperial Landfill",
    "actual": "Imperial Landfill",
    "score": 0.86,
    "match_type": "semantic"
  },
  "quantity": {
    "expected": 18.5,
    "actual": 185,
    "score": 0.0,
    "match_type": "numeric_mismatch"
  }
}

```

---

# 3. Endpoint Design

You want a clean API that enterprise customers can integrate with easily.

## Auth

Every request uses:

```http
Authorization: Bearer <api_key>
X-Org-Id: org_123

```

For idempotent ingestion:

```http
Idempotency-Key: customer-doc-abc-123

```

---

# 4. Public API Endpoints

## Upload and start ingestion

```http
POST /v1/projects/{project_id}/documents/ingest
Content-Type: multipart/form-data

```

Request:

```txt
file: manifest.pdf
pipeline_id: pipe_manifest_v3
schema_id: schema_manifest_v7
model_config_id: model_gemini_manifest_v4
async: true
callback_url: https://customer.com/webhooks/document-result

```

Response:

```json
{
  "document_id": "doc_123",
  "ingestion_run_id": "run_456",
  "status": "queued",
  "trace_id": "trace_abc"
}

```

This is the main endpoint.

---

## Create document from existing S3 URL

For enterprise customers who do not want multipart uploads.

```http
POST /v1/projects/{project_id}/documents

```

Request:

```json
{
  "source_uri": "s3://customer-bucket/manifests/123.pdf",
  "filename": "manifest-123.pdf",
  "checksum_sha256": "abc123",
  "metadata": {
    "customer_id": "cust_991",
    "job_id": "job_812",
    "facility": "Imperial"
  }
}

```

Response:

```json
{
  "document_id": "doc_123",
  "status": "created"
}

```

---

## Start a run on an existing document

```http
POST /v1/documents/{document_id}/runs

```

Request:

```json
{
  "pipeline_id": "pipe_manifest_v3",
  "schema_id": "schema_manifest_v7",
  "model_config_id": "model_gemini_manifest_v4",
  "prompt_version_id": "prompt_manifest_v12",
  "callback_url": "https://customer.com/webhooks/document-result"
}

```

Response:

```json
{
  "ingestion_run_id": "run_456",
  "status": "queued"
}

```

---

## Get run status

```http
GET /v1/runs/{run_id}

```

Response:

```json
{
  "id": "run_456",
  "document_id": "doc_123",
  "status": "completed",
  "started_at": "2026-05-08T14:02:11Z",
  "completed_at": "2026-05-08T14:02:27Z",
  "trace_id": "trace_abc",
  "validation_status": "passed",
  "confidence_score": 0.91,
  "ground_truth_score": null
}

```

---

## Get structured output

```http
GET /v1/runs/{run_id}/output

```

Response:

```json
{
  "run_id": "run_456",
  "schema_id": "schema_manifest_v7",
  "model_config_id": "model_gemini_manifest_v4",
  "prompt_version_id": "prompt_manifest_v12",
  "output": {
    "manifest_number": "MNF-102991",
    "generator_name": "Range Resources",
    "destination_facility": "Imperial Landfill",
    "quantity": 18.5,
    "unit": "tons"
  },
  "field_confidence": {
    "manifest_number": 0.98,
    "generator_name": 0.92,
    "destination_facility": 0.87,
    "quantity": 0.94
  },
  "validation": {
    "status": "passed",
    "errors": []
  }
}

```

---

## Re-run with another model

This is central to your platform.

```http
POST /v1/runs/{run_id}/rerun

```

Request:

```json
{
  "model_config_id": "model_openai_manifest_v2",
  "prompt_version_id": "prompt_manifest_v12",
  "reason": "Compare OpenAI against Gemini baseline"
}

```

Response:

```json
{
  "new_ingestion_run_id": "run_789",
  "parent_run_id": "run_456",
  "status": "queued"
}

```

---

## Compare runs

```http
POST /v1/runs/compare

```

Request:

```json
{
  "run_ids": ["run_456", "run_789", "run_999"]
}

```

Response:

```json
{
  "document_id": "doc_123",
  "comparison": [
    {
      "run_id": "run_456",
      "provider": "gemini",
      "model": "gemini-2.5-pro",
      "confidence_score": 0.91,
      "validation_status": "passed",
      "latency_ms": 14200,
      "estimated_cost": 0.081
    },
    {
      "run_id": "run_789",
      "provider": "openai",
      "model": "gpt-4.1",
      "confidence_score": 0.88,
      "validation_status": "passed",
      "latency_ms": 9700,
      "estimated_cost": 0.124
    }
  ],
  "field_differences": {
    "destination_facility": [
      {
        "run_id": "run_456",
        "value": "Imperial Landfill"
      },
      {
        "run_id": "run_789",
        "value": "Republic Imperial Landfill"
      }
    ]
  }
}

```

---

## Submit ground truth

```http
POST /v1/documents/{document_id}/ground-truth

```

Request:

```json
{
  "schema_id": "schema_manifest_v7",
  "expected_output": {
    "manifest_number": "MNF-102991",
    "generator_name": "Range Resources",
    "destination_facility": "Republic Imperial Landfill",
    "quantity": 18.5,
    "unit": "tons"
  }
}

```

Response:

```json
{
  "ground_truth_id": "gt_123",
  "status": "created"
}

```

---

## Evaluate run against ground truth

```http
POST /v1/runs/{run_id}/evaluate

```

Response:

```json
{
  "evaluation_result_id": "eval_123",
  "total_score": 0.92,
  "field_scores": {
    "manifest_number": 1.0,
    "generator_name": 1.0,
    "destination_facility": 0.86,
    "quantity": 1.0,
    "unit": 1.0
  },
  "missing_required_fields": [],
  "hallucinated_fields": [],
  "validation_errors": []
}

```

---

## Batch eval

For prompt/model testing.

```http
POST /v1/evaluations/batch

```

Request:

```json
{
  "dataset_id": "dataset_manifest_001",
  "model_config_ids": [
    "model_gemini_manifest_v4",
    "model_openai_manifest_v2",
    "model_claude_manifest_v1"
  ],
  "prompt_version_ids": [
    "prompt_manifest_v12",
    "prompt_manifest_v13"
  ],
  "schema_id": "schema_manifest_v7"
}

```

Response:

```json
{
  "batch_evaluation_id": "batch_eval_123",
  "status": "queued",
  "total_runs": 600
}

```

---

## Get batch eval results

```http
GET /v1/evaluations/batch/{batch_evaluation_id}

```

Response:

```json
{
  "id": "batch_eval_123",
  "status": "completed",
  "summary": {
    "best_model_config_id": "model_gemini_manifest_v4",
    "best_prompt_version_id": "prompt_manifest_v13",
    "average_score": 0.947,
    "average_latency_ms": 12800,
    "average_cost": 0.074
  },
  "results": [
    {
      "model_config_id": "model_gemini_manifest_v4",
      "prompt_version_id": "prompt_manifest_v13",
      "average_score": 0.947,
      "required_field_accuracy": 0.981,
      "hallucination_rate": 0.012,
      "average_latency_ms": 12800,
      "average_cost": 0.074
    }
  ]
}

```

---

# 5. Internal Workflow

Temporal workflow:

```txt
DocumentIngestionWorkflow
  1. ValidateDocumentActivity
  2. StoreDocumentMetadataActivity
  3. ExtractPagesActivity
  4. DetectDocumentTypeActivity
  5. SelectPipelineActivity
  6. RunModelExtractionActivity
  7. ParseModelOutputActivity
  8. NormalizeOutputActivity
  9. ValidateSchemaActivity
  10. ScoreConfidenceActivity
  11. EvaluateAgainstGroundTruthActivity
  12. PersistOutputActivity
  13. PublishWebhookActivity

```

Each activity should be independently retryable.

Example retries:


| Step            | Retry?     | Notes                              |
| --------------- | ---------- | ---------------------------------- |
| Store file      | Yes        | Idempotent by checksum             |
| Extract pages   | Yes        | Deterministic                      |
| Call model      | Yes        | With provider timeout              |
| Parse output    | No/limited | Bad prompt/schema should fail fast |
| Validate schema | No         | Deterministic                      |
| Webhook         | Yes        | Exponential backoff                |
| Evaluation      | Yes        | Deterministic                      |


---

# 6. Provider Abstraction

You need a unified interface.

```ts
interface DocumentModelProvider {
  extract(input: ExtractionInput): Promise<ExtractionResult>
}

```

Input:

```ts
type ExtractionInput = {
  documentUri: string
  pages?: number[]
  mimeType: string
  prompt: string
  schema: JsonSchema
  modelConfig: ModelConfig
  metadata: Record<string, unknown>
}

```

Output:

```ts
type ExtractionResult = {
  provider: string
  model: string
  rawOutput: unknown
  parsedOutput: Record<string, unknown>
  usage: {
    inputTokens?: number
    outputTokens?: number
    pagesProcessed?: number
  }
  latencyMs: number
  estimatedCost: number
  warnings: string[]
}

```

Then you implement adapters:

```txt
GeminiProvider
OpenAIProvider
ClaudeProvider
TextractProvider
GoogleDocumentAIProvider
AzureDocumentIntelligenceProvider
LocalModelProvider

```

Do not leak provider-specific quirks into the product.

The product should see this:

```json
{
  "provider": "gemini",
  "model": "gemini-2.5-pro",
  "parsed_output": {}
}

```

Not this:

```json
{
  "candidates": [],
  "content": {},
  "finishReason": "STOP"
}

```

Raw provider output goes to object storage for debugging.

---

# 7. Hot Switching Logic

Hot switching should not just mean “pick another provider.”

You need routing policies.

## Model Routing Policy

```ts
ModelRoutingPolicy {
  id
  project_id
  name
  strategy
  primary_model_config_id
  fallback_model_config_ids
  min_confidence_threshold
  max_latency_ms
  max_cost_per_document
}

```

Strategies:

```txt
primary_only
fallback_on_failure
fallback_on_low_confidence
parallel_compare
cheapest_that_passes
highest_accuracy
customer_specific_best_model

```

Example:

```json
{
  "strategy": "fallback_on_low_confidence",
  "primary_model_config_id": "model_gemini_manifest_v4",
  "fallback_model_config_ids": [
    "model_openai_manifest_v2",
    "model_claude_manifest_v1"
  ],
  "min_confidence_threshold": 0.88
}

```

Workflow:

```txt
Run primary model
    ↓
Validate required fields
    ↓
Confidence < threshold?
    ↓
Run fallback model
    ↓
Compare outputs
    ↓
Choose winner or flag for human review

```

---

# 8. Human Review Layer

This matters for selling enterprise.

You need a review queue.

```http
GET /v1/review-queue?project_id=...

```

A run goes to human review when:

```txt
schema validation fails
required fields missing
confidence below threshold
models disagree on important fields
ground truth regression detected
customer rule fails

```

Review item:

```ts
ReviewItem {
  id
  ingestion_run_id
  document_id
  reason
  priority
  assigned_to
  status
  corrected_output_json
  created_at
}

```

Once reviewed, the corrected result can become ground truth.

This creates the learning loop.

```txt
Failed extraction
→ human correction
→ ground truth
→ future evaluation set
→ model/prompt improvement

```

That is a moat.

---

# 9. Observability Architecture

You need observability at four levels:

1. **System health**
2. **Workflow health**
3. **Model/provider health**
4. **Output quality**

Most people only monitor level 1. Your product value is levels 3 and 4.

---

## Observability Stack

```txt
OpenTelemetry SDK
    ↓
OTel Collector
    ↓
Traces: Tempo or Jaeger
Logs: Loki
Metrics: Prometheus
Dashboards: Grafana
Errors: Sentry optional

```

---

# 10. Required Trace Structure

Every ingestion run needs a trace.

Trace name:

```txt
document.ingestion

```

Trace attributes:

```txt
org.id
project.id
document.id
ingestion_run.id
pipeline.id
schema.id
provider.name
model.name
prompt.version
document.mime_type
document.page_count
document.checksum

```

Spans:

```txt
document.upload
document.store
document.page_extract
document.type_detect
model.extract
model.parse
schema.validate
confidence.score
ground_truth.evaluate
output.persist
webhook.publish

```

This lets you answer:

```txt
Why did this document take 48 seconds?
Was it model latency?
Was it page extraction?
Was it webhook delivery?
Was it schema validation?

```

---

# 11. Metrics You Need

## API Metrics

```txt
http_requests_total
http_request_duration_ms
http_errors_total
api_rate_limited_total
api_auth_failures_total

```

Labels:

```txt
endpoint
method
status_code
organization_id
project_id

```

---

## Document Metrics

```txt
documents_ingested_total
document_pages_processed_total
document_upload_bytes_total
document_processing_duration_ms
document_processing_failures_total

```

Labels:

```txt
org_id
project_id
document_type
mime_type
pipeline_id

```

---

## Workflow Metrics

```txt
ingestion_runs_total
ingestion_runs_completed_total
ingestion_runs_failed_total
ingestion_runs_retried_total
ingestion_workflow_duration_ms
activity_duration_ms
activity_failures_total

```

Labels:

```txt
pipeline_id
activity_name
status
error_code

```

---

## Model Metrics

```txt
model_requests_total
model_request_duration_ms
model_failures_total
model_timeouts_total
model_retries_total
model_input_tokens_total
model_output_tokens_total
model_estimated_cost_total

```

Labels:

```txt
provider
model
prompt_version
schema_id
project_id

```

---

## Quality Metrics

These are the money metrics.

```txt
extraction_confidence_avg
schema_validation_failure_rate
required_field_missing_rate
field_accuracy_score
ground_truth_match_score
hallucination_rate
human_review_rate
model_disagreement_rate

```

Labels:

```txt
project_id
schema_id
field_name
provider
model
prompt_version
document_type

```

This lets you show customers:

```txt
Manifest Number Accuracy: 99.2%
Destination Facility Accuracy: 94.7%
Quantity Accuracy: 91.8%
Human Review Rate: 6.4%
Average Cost Per Document: $0.083
Average Processing Time: 14.2s

```

That is what makes the platform sellable.

---

# 12. Logs

Use structured JSON logs only.

Example:

```json
{
  "level": "info",
  "event": "model_extraction_completed",
  "organization_id": "org_123",
  "project_id": "proj_123",
  "document_id": "doc_123",
  "ingestion_run_id": "run_456",
  "provider": "gemini",
  "model": "gemini-2.5-pro",
  "prompt_version_id": "prompt_manifest_v12",
  "schema_id": "schema_manifest_v7",
  "latency_ms": 14200,
  "estimated_cost": 0.081,
  "trace_id": "trace_abc"
}

```

Bad log:

```txt
Model finished successfully

```

Worthless.

---

# 13. Dashboards

## Dashboard 1: Executive / Customer-Facing

Show this to customers.

```txt
Documents processed
Success rate
Human review rate
Average confidence
Average processing time
Estimated money saved
Field-level accuracy
Provider comparison

```

Example:


| Metric                      | Value  |
| --------------------------- | ------ |
| Documents processed         | 42,180 |
| Straight-through processing | 91.4%  |
| Human review rate           | 8.6%   |
| Avg confidence              | 93.2%  |
| Avg processing time         | 13.8s  |
| Validation failure rate     | 2.1%   |


---

## Dashboard 2: Model Performance

Internal and customer admin.

```txt
Accuracy by provider
Accuracy by prompt version
Cost by provider
Latency by provider
Failure rate by model
Field-level disagreement
Regression alerts

```

Example:


| Provider | Score | Latency | Cost/doc | Failure Rate |
| -------- | ----- | ------- | -------- | ------------ |
| Gemini   | 94.7% | 14.2s   | $0.08    | 1.2%         |
| OpenAI   | 92.1% | 9.7s    | $0.12    | 0.8%         |
| Claude   | 93.4% | 17.8s   | $0.15    | 0.5%         |
| Textract | 81.9% | 4.2s    | $0.03    | 3.8%         |


---

## Dashboard 3: Pipeline Health

For engineering.

```txt
Queue depth
Workflow failures
Activity retries
Provider timeouts
Webhook failures
DB latency
Object storage latency

```

---

## Dashboard 4: Field Accuracy

This is the killer dashboard.


| Field                | Accuracy | Missing Rate | Disagreement Rate | Human Review Rate |
| -------------------- | -------- | ------------ | ----------------- | ----------------- |
| Manifest Number      | 99.2%    | 0.3%         | 0.8%              | 1.1%              |
| Generator Name       | 96.8%    | 1.7%         | 4.2%              | 5.9%              |
| Destination Facility | 94.7%    | 2.8%         | 7.6%              | 9.1%              |
| Quantity             | 91.8%    | 3.4%         | 11.2%             | 12.7%             |


This is what proves model reliability.

---

# 14. Alerts

You need alerts for both infra and quality.

## Infrastructure Alerts

```txt
API 5xx rate > 2% for 5 min
Workflow failure rate > 5% for 10 min
Queue depth > threshold
Model provider timeout rate > 10%
Webhook failure rate > 15%
DB connection saturation
Object storage upload failures

```

## Quality Alerts

These are more valuable.

```txt
Required field missing rate increased by 20%
Ground truth score dropped below threshold
Provider output changed significantly
Prompt version regression detected
Human review rate increased sharply
Model disagreement rate crossed threshold
Cost per document exceeded threshold
Latency p95 exceeded SLA

```

Example alert:

```txt
ALERT: Destination facility accuracy dropped from 94.7% to 82.1%
Project: DTE CCR Ticket Processing
Model: gemini-2.5-pro
Prompt: manifest-v13
Detected after 250 documents
Suggested action: rollback to prompt manifest-v12

```

That is enterprise-grade.

---

# 15. Database Tables

Minimum schema:

```txt
organizations
projects
api_keys

documents
document_pages
ingestion_runs
pipeline_definitions
pipeline_versions

model_providers
model_configs
prompt_versions
extraction_schemas

raw_model_outputs
normalized_outputs
validation_results
ground_truths
evaluation_results
batch_evaluations

review_items
review_decisions

webhook_endpoints
webhook_deliveries

audit_logs

```

---

# 16. Status Model

Use explicit statuses.

```txt
document.status:
uploaded
stored
processing
completed
failed

ingestion_run.status:
queued
running
waiting_on_provider
validating
requires_review
completed
failed
cancelled

review_item.status:
open
assigned
corrected
approved
rejected
closed

```

---

# 17. Event Model

Emit internal events.

```txt
document.uploaded
ingestion_run.created
ingestion_run.started
model.extraction.started
model.extraction.completed
schema.validation.failed
ground_truth.evaluation.completed
review.required
review.completed
ingestion_run.completed
webhook.delivered

```

Event shape:

```json
{
  "event_id": "evt_123",
  "event_type": "ingestion_run.completed",
  "organization_id": "org_123",
  "project_id": "proj_123",
  "document_id": "doc_123",
  "ingestion_run_id": "run_456",
  "occurred_at": "2026-05-08T14:02:27Z",
  "payload": {}
}

```

Later, you can stream these to customers.

---

# 18. Webhooks

Customers need results pushed back.

```http
POST https://customer.com/webhooks/document-result

```

Payload:

```json
{
  "event_type": "ingestion_run.completed",
  "document_id": "doc_123",
  "ingestion_run_id": "run_456",
  "status": "completed",
  "output_url": "https://api.yourapp.com/v1/runs/run_456/output",
  "confidence_score": 0.91,
  "validation_status": "passed"
}

```

Webhook delivery table:

```ts
WebhookDelivery {
  id
  endpoint_id
  event_id
  status
  attempt_count
  last_attempt_at
  next_retry_at
  response_status_code
  response_body
}

```

Retries:

```txt
1 min
5 min
15 min
1 hour
6 hours
24 hours

```

---

# 19. Idempotency

Critical.

Customer systems will retry uploads.

For every ingestion request, support:

```http
Idempotency-Key: customer-system-doc-123

```

You store:

```ts
IdempotencyKey {
  key
  organization_id
  request_hash
  response_json
  created_at
  expires_at
}

```

If same key and same request:

```txt
return original response

```

If same key and different request:

```txt
409 Conflict

```

---

# 20. Security / Compliance Basics

You need:

```txt
API key auth
org/project-level scoping
encrypted object storage
signed URLs
audit logs
least-privilege service roles
PII redaction option
data retention policies
customer-specific deletion
access logs

```

Audit log:

```ts
AuditLog {
  id
  organization_id
  actor_type
  actor_id
  action
  resource_type
  resource_id
  metadata
  created_at
}

```

Examples:

```txt
document.uploaded
run.output.viewed
ground_truth.updated
prompt_version.created
model_config.changed
schema.changed

```

---

# 21. MVP Version

Do not build the full monster first.

Build this MVP:

## MVP Features

```txt
1. Upload document
2. Run extraction through one provider
3. Store raw + normalized output
4. Validate JSON schema
5. Add ground truth manually
6. Re-run with another provider/prompt
7. Compare outputs
8. Basic dashboard
9. Webhook result delivery
10. Trace every run

```

## MVP Stack

```txt
FastAPI
Postgres
S3
Temporal
Python workers
OpenTelemetry
Grafana Cloud or self-hosted Grafana stack

```

Skip initially:

```txt
Kafka
Rust
complex routing policies
advanced semantic scoring
multi-tenant billing
plugin marketplace
fine-tuning
custom model hosting

```

---

# 22. The First Sellable Product Shape

I would package it as:

## “Document AI Reliability Layer”

Core pitch:

> Standardize document extraction across AI providers, test prompt/model changes against ground truth, and guarantee stable structured outputs before they hit downstream systems.

The buyer-facing modules:

```txt
1. Ingestion API
2. Model Router
3. Schema Validator
4. Ground Truth Evaluator
5. Human Review Queue
6. Output Webhooks
7. Reliability Dashboard

```

What they pay for:

```txt
reduce manual review
reduce bad downstream data
prevent model regressions
compare providers
switch models safely
maintain auditability

```

---

# 23. The Architecture I’d Actually Build First

```txt
                    ┌────────────────────┐
                    │ Customer System     │
                    └─────────┬──────────┘
                              │
                              ▼
                    ┌────────────────────┐
                    │ Ingestion API       │
                    │ FastAPI/NestJS      │
                    └─────────┬──────────┘
                              │
                ┌─────────────┴─────────────┐
                ▼                           ▼
       ┌────────────────┐          ┌────────────────┐
       │ Postgres        │          │ S3              │
       │ metadata/state  │          │ raw files/output│
       └────────────────┘          └────────────────┘
                │
                ▼
       ┌────────────────┐
       │ Temporal        │
       │ workflow engine │
       └───────┬────────┘
               │
               ▼
       ┌────────────────┐
       │ Python Workers  │
       │ extraction/evals│
       └───────┬────────┘
               │
   ┌───────────┼───────────────┐
   ▼           ▼               ▼
Gemini      OpenAI          Textract
Claude      Azure Doc AI    Local Models

               │
               ▼
       ┌────────────────┐
       │ Validation +    │
       │ Evaluation      │
       └───────┬────────┘
               │
               ▼
       ┌────────────────┐
       │ Output API +    │
       │ Webhooks        │
       └────────────────┘

Observability across all layers:
OpenTelemetry → Tempo/Jaeger
Logs → Loki
Metrics → Prometheus
Dashboards → Grafana
Errors → Sentry

```

---

# 24. The Real Moat

This platform becomes valuable when you can show:

```txt
For this customer's actual documents:
- Model A is best for handwritten manifests
- Model B is cheaper for printed invoices
- Prompt v14 improved generator extraction but hurt quantity extraction
- Schema v6 reduced validation failures by 37%
- Human review corrections improved future evals
- Provider regression was detected before production rollout

```

That is not an OCR wrapper.

That is operational AI infrastructure.