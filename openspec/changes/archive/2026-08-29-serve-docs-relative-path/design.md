# Design: Serve Documentation Using Relative Paths

## Context

The `store_order` microservice serves interactive documentation via Chi router:
- Scalar UI mounted at `/docs`
- Swagger UI mounted at `/swagger`
- OpenAPI 3.1 YAML/JSON specs at `/docs/openapi.yaml` and `/docs/openapi.json`

Currently, the HTML responses hardcode absolute root paths:
- `<script id="api-reference" data-url="/docs/openapi.json"></script>`
- `SwaggerUIBundle({ url: "/docs/openapi.json", ... })`

Additionally, `docs/openapi.yaml` and `docs/openapi.json` declare servers as:
```yaml
servers:
  - url: http://localhost:8060
  - url: /
```

When accessed through a subpath or API Gateway prefix (e.g. `https://api.store.com/orders/docs`), the browser resolves `/docs/openapi.json` against the host root, yielding HTTP 404s.

## Goals / Non-Goals

**Goals:**
- Enable interactive documentation to load correctly when accessed directly or via reverse proxies/gateways with arbitrary path prefixes.
- Update Scalar UI to resolve `./openapi.json`.
- Normalize `/docs` to `/docs/` to guarantee RFC 3986 relative path resolution targets `/docs/openapi.json`.
- Update Swagger UI to resolve the spec relatively.
- Add relative server entry `url: ./` in both OpenAPI 3.1 YAML and JSON files.
- Add automated router tests verifying relative paths and HTTP 200 responses.

**Non-Goals:**
- Self-hosting vendor CSS/JS bundles (CDN-delivered assets remain in use).
- Breaking existing API routes or changing spec schema contents.

## Decisions

### Decision 1: Trailing Slash Normalization for `/docs`
- **Choice**: Redirect `/docs` to `/docs/` using `http.StatusMovedPermanently`.
- **Rationale**: In RFC 3986 URL resolution:
  - Base `http://host/orders/docs` + `./openapi.json` ➔ `http://host/orders/openapi.json` (404)
  - Base `http://host/orders/docs/` + `./openapi.json` ➔ `http://host/orders/docs/openapi.json` (200 OK)
  Redirecting to `/docs/` ensures deterministic browser resolution.
- **Alternatives Considered**:
  - HTML `<base>` tag: Can interfere with fragment links (`#tag/operations`) used heavily by Scalar and Swagger.
  - Custom JavaScript URL parsing: Adds unnecessary code complexity compared to a standard HTTP redirect.

### Decision 2: Relative Target for Scalar UI
- **Choice**: Change `data-url="/docs/openapi.json"` to `data-url="./openapi.json"`.
- **Rationale**: When rendered at `/docs/`, `./openapi.json` resolves directly to `/docs/openapi.json` regardless of any preceding gateway path prefix.

### Decision 3: Relative Target for Swagger UI
- **Choice**: Update Swagger UI to use `url: "./docs/openapi.json"` (when accessed at `/swagger`) and support `/swagger/` redirect with `url: "../docs/openapi.json"`, or alias `/docs/swagger`.
- **Rationale**: Ensures Swagger UI loads the spec without absolute root dependencies.

### Decision 4: OpenAPI Servers Relative URL
- **Choice**: Add `- url: ./` with description `"Current Gateway / Relative Origin"` as the primary or secondary entry in `docs/openapi.yaml` and `docs/openapi.json`.
- **Rationale**: OpenAPI 3.1 specification explicitly supports relative server URLs. This allows the interactive test console to execute requests against the current path base.

## Risks / Trade-offs

- **[Risk] Reverse proxy stripping trailing slash on redirect**
  - **Mitigation**: Use relative redirect `r.URL.Path + "/"` so proxies preserve upstream path prefixes.
- **[Risk] Schema synchronization drift**
  - **Mitigation**: Update both `docs/openapi.yaml` and `docs/openapi.json` in tandem and validate syntax.
