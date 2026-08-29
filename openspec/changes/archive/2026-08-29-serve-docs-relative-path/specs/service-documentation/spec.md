## MODIFIED Requirements

### Requirement: OpenAPI Specification Parity and Interactive Documentation
The system SHALL maintain synchronized and valid OpenAPI 3.1 specifications in both YAML (`docs/openapi.yaml`) and JSON (`docs/openapi.json`) formats covering 100% of available HTTP routes, authentication headers, rate limit response headers, error models, dev simulation endpoints, and relative server definitions (`./`).

#### Scenario: Client queries OpenAPI spec endpoints
- **WHEN** a client or automated tool fetches `/docs/openapi.yaml` or `/docs/openapi.json`
- **THEN** the returned specification SHALL accurately describe all paths (`/health`, `/api/v1/orders`, `/api/v1/orders/{id}`, `/api/v1/orders/{id}/cancel`, `/api/v1/orders/webhook/midtrans`, `/api/v1/admin/orders`, `/api/v1/admin/orders/{id}/status`, `/api/v1/dev/orders/*`) with complete parameter and response schemas, and SHALL include a relative server URL (`./`) in the `servers` configuration

#### Scenario: Developer navigates interactive API documentation
- **WHEN** a developer accesses `/docs` (Scalar UI) or `/swagger` (Swagger UI) in a browser while `ENABLE_DOCS=true`
- **THEN** the interactive UI SHALL render all documented endpoints with schema validation, example payloads, and authentication parameter descriptions, AND SHALL load the OpenAPI specification via relative path references (`./openapi.json` or relative paths) to support arbitrary reverse proxy prefixes and sub-paths

#### Scenario: Trailing slash normalization for documentation
- **WHEN** a client accesses `/docs` without a trailing slash
- **THEN** the router SHALL redirect or normalize the request to `/docs/` (or ensure RFC 3986 path resolution targets `/docs/openapi.json`)
