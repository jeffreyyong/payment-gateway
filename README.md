# payment-gateway

## Description

This service is a REST API based application that allows a merchant to offer a way for their shoppers
to pay for their product.

It provides 4 payment actions: authorizate, void, capture and refund.


## How It Works
All of the endpoints require request_id for idempotency
### Authorize
- POST /authorize
- Authorization only happens during the transaction creation
- sample JSON request body:
  ```json
  {
    "request_id": "48adc9e8-cb79-42d3-8ebf-e628d2bf86ca",
    "payment_source": {
        "pan": "4000000000000259",
        "cvv": "123",
        "expiry_month": 1,
        "expiry_year": 21
    },
    "amount": {
        "minor_units": 10555,
        "currency": "GBP",
        "exponent": 2
    },
    "description": "APPLE.COM",
  }
  ```

### Void
- POST /void
- Void will cancel the tranasction and no other payment actions possible after it's voided.
- sample JSON request body:
  ```json
  {
    "request_id": "c146222f-c311-4f48-afc4-a762c73fbe65",
    "authorization_id": "e47ab09a-18e0-45c3-b113-abc01319373a"
  }
  ```

### Capture
- POST /capture
- Capture can triggered multiple times as long as the amount is less than the authorized amount.
- sample JSON request body:
  ```json
  {
    "request_id": "c146222f-c311-4f48-afc4-a762c73fbe65",
    "amount": {
        "minor_units": 5555,
        "currency": "GBP",
        "exponent": 2
    },
    "authorization_id": "e47ab09a-18e0-45c3-b113-abc01319373a"
  }
  ```
  
### Refund
- POST /refund
- Refund can be triggered multiple times as long as the amount is less than the captured amount.
- Capture cannot be made on a transaction after it's refunded.
- sample JSON request body:
  ```json
  {
    "request_id": "c146222f-c311-4f48-afc4-a762c73fbe65",
    "amount": {
        "minor_units": 5555,
        "currency": "GBP",
        "exponent": 2
    },
    "authorization_id": "e47ab09a-18e0-45c3-b113-abc01319373a"
  }
  ```


## Local Development
- Dockerfile has been provided to containerized the application and PostgreSQL DB
```shell
go mod tidy
make regenerate
make run-docker
make test-all
```