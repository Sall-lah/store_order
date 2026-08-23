## ADDED Requirements

### Requirement: Development Environment Guarding
The system SHALL conditionally enable developer simulation endpoints and mock Snap token generation only when the environment variable `DEV` evaluates to `true` (case-insensitive).

#### Scenario: Dev endpoints active in development
- **WHEN** `DEV="TRUE"` is configured in `.env`
- **THEN** the router registers `/api/v1/dev/*` simulation endpoints and enables fallback mock payment token generation

#### Scenario: Dev endpoints disabled in non-development
- **WHEN** `DEV="FALSE"` or `DEV` is unset or production mode is active
- **THEN** requests to `/api/v1/dev/*` return HTTP 404 Not Found, and mock token generation is strictly disabled

### Requirement: Order Payment Simulation
The system SHALL provide simulation endpoints under `/api/v1/dev/orders` allowing developers to trigger order status transitions (`PAID`, `CANCELLED`, `EXPIRED`) offline.

#### Scenario: Simulate payment success
- **WHEN** a client sends `POST /api/v1/dev/orders/{id}/simulate-success` in development mode
- **THEN** the system transitions the order to `PAID`, populates payment timestamps, persists an `order.paid` outbox event, and returns HTTP 200 with the updated order

#### Scenario: Simulate order cancellation
- **WHEN** a client sends `POST /api/v1/dev/orders/{id}/simulate-cancel` in development mode
- **THEN** the system transitions the order to `CANCELLED`, persists an `order.cancelled` outbox event, and returns HTTP 200

#### Scenario: Simulate order expiration
- **WHEN** a client sends `POST /api/v1/dev/orders/{id}/simulate-expire` in development mode
- **THEN** the system transitions the order to `EXPIRED`, persists an `order.expired` outbox event, and returns HTTP 200
