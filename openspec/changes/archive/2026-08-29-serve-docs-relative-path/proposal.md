# Proposal: Serve Documentation Using Relative Paths

## Why

Currently, the interactive API documentation endpoints (Scalar UI at `/docs` and Swagger UI at `/swagger`) hardcode absolute root paths (`/docs/openapi.json`), and the OpenAPI specification servers define only absolute roots (`/` and `http://localhost:8060`). 

When `store_order` is hosted behind an API Gateway, Kubernetes Ingress, or reverse proxy using path-based prefixes (e.g., `https://api.store.com/orders/docs`), browser requests for `/docs/openapi.json` bypass the path prefix and query the host root, resulting in HTTP 404 errors. Serving documentation assets and OpenAPI specs using relative paths (`./`) enables seamless documentation rendering and interactive API testing regardless of prefix mounting or proxy configuration.

## What Changes

- **Scalar UI Relative Specification Target**: Update the Scalar UI HTML in `internal/router/router.go` from `data-url="/docs/openapi.json"` to relative path `./openapi.json`.
- **Trailing Slash Normalization**: Ensure `/docs` redirects to `/docs/` (or registers both `/docs` and `/docs/`) so that RFC 3986 relative path resolution reliably resolves `./openapi.json` to `/docs/openapi.json`.
- **Swagger UI Relative Specification Target**: Update Swagger UI HTML in `internal/router/router.go` to resolve the OpenAPI specification using relative path (e.g., `../docs/openapi.json` or `./openapi.json`).
- **OpenAPI Server Entries**: Update `docs/openapi.yaml` and `docs/openapi.json` to include `./` (Relative Base / Gateway) in the `servers` definition list.
- **Router Test Coverage**: Add unit tests in `internal/router/router_test.go` verifying that `/docs`, `/docs/`, and `/swagger` respond with status 200 and deliver HTML containing relative specification links.

## Capabilities

### New Capabilities
*(None)*

### Modified Capabilities
- `service-documentation`: Update the interactive API documentation requirement to require relative path specification references (`./`) for Scalar and Swagger UIs and relative base URLs in the OpenAPI server configuration.

## Impact

- **Affected Code**: `internal/router/router.go`, `internal/router/router_test.go`
- **Affected API Specs**: `docs/openapi.yaml`, `docs/openapi.json`
- **Specification Delta**: `openspec/specs/service-documentation/spec.md`
- **Breaking Changes**: None. Relative path resolution preserves backwards compatibility for standard local development on `http://localhost:8060/docs`.
