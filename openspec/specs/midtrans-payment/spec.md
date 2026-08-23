# Midtrans Payment Specification

## Purpose
Governs integration with the Midtrans Snap Payment Gateway, transaction token creation, and cryptographic verification of asynchronous webhook notifications.

## Requirements

### Requirement: Midtrans Snap Token Generation
The system SHALL interact with the Midtrans Snap API during order creation to generate a secure transaction token and payment redirect URL using configured server credentials.

#### Scenario: Successful Snap token issuance
- **WHEN** an order is created and Midtrans API credentials are configured
- **THEN** the system generates a unique transaction payload with gross amount and customer details, requests a Snap token from Midtrans, and stores the token and redirect URL on the order record

#### Scenario: Midtrans API error handling
- **WHEN** the Midtrans Snap API returns an error or is unreachable
- **THEN** the system fails the checkout transaction gracefully with HTTP 502 Bad Gateway and rolls back order persistence

### Requirement: Midtrans Webhook Notification Handling
The system SHALL expose a public HTTP endpoint `POST /api/v1/orders/webhook/midtrans` to receive and process asynchronous payment notifications from Midtrans.

#### Scenario: Verified settlement notification
- **WHEN** Midtrans sends a notification with `transaction_status: settlement` or `capture` with `fraud_status: accept` and a valid SHA512 signature matching `SHA512(order_id + status_code + gross_amount + server_key)`
- **THEN** the system updates the corresponding order status to `PAID`, sets `paid_at` timestamp, atomically inserts an `order.paid` event into the outbox table, and responds with HTTP 200 OK

#### Scenario: Invalid signature notification
- **WHEN** a webhook request is received with an invalid or missing signature key
- **THEN** the system rejects the notification with HTTP 401 Unauthorized without modifying order state

#### Scenario: Expired or Cancelled payment notification
- **WHEN** Midtrans sends a notification with `transaction_status: expire` or `cancel` with a valid signature
- **THEN** the system updates the order status to `EXPIRED` or `CANCELLED`, records an outbox event, and responds with HTTP 200 OK

#### Scenario: Idempotent notification processing
- **WHEN** Midtrans delivers a duplicate webhook notification for an order already transitioned to `PAID`
- **THEN** the system acknowledges the notification with HTTP 200 OK without re-inserting duplicate outbox events or mutating final state
