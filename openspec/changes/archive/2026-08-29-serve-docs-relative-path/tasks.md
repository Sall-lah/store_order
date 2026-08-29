# Tasks: Serve Documentation Using Relative Paths

## 1. Documentation Routes & UI Updates

- [x] 1.1 Update Scalar UI HTML in `internal/router/router.go` to reference `./openapi.json` via `data-url`
- [x] 1.2 Implement trailing-slash normalization for `/docs` (redirecting `/docs` to `/docs/`) to ensure RFC 3986 relative path resolution
- [x] 1.3 Mount `/docs/` route to serve Scalar UI
- [x] 1.4 Update Swagger UI HTML in `internal/router/router.go` to resolve the OpenAPI specification using relative path

## 2. OpenAPI Specification Updates

- [x] 2.1 Update `docs/openapi.yaml` `servers` section to include `- url: ./` with description `"Current Gateway / Relative Origin"`
- [x] 2.2 Synchronize `docs/openapi.json` `servers` section to include `{ "url": "./", "description": "Current Gateway / Relative Origin" }`

## 3. Router Tests & Verification

- [x] 3.1 Add router unit tests in `internal/router/router_test.go` verifying documentation endpoints (`/docs`, `/docs/`, `/swagger`, `/docs/openapi.json`)
- [x] 3.2 Verify `/docs` redirection to `/docs/` and verify `/docs/` contains `data-url="./openapi.json"`
- [x] 3.3 Verify `/swagger` contains relative specification URL
- [x] 3.4 Execute full test suite `go test -v ./internal/router/...` to validate implementation
