CREATE TYPE payment_action_type AS ENUM ('authorization', 'void', 'capture', 'refund');
CREATE TYPE payment_action_status AS ENUM('success', 'failed', 'pending');
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";


CREATE TABLE IF NOT EXISTS card
(
    id           UUID        NOT NULL PRIMARY KEY DEFAULT uuid_generate_v4(),
    pan          VARCHAR(19) NOT NULL UNIQUE,
    cvv          VARCHAR(4)  NOT NULL,
    expiry_month VARCHAR(2)  NOT NULL,
    expiry_year  VARCHAR(2)  NOT NULL,
    created_date TIMESTAMPTZ NOT NULL,
    updated_date TIMESTAMPTZ NOT NULL

);

-- TODO: add search indexes
CREATE TABLE IF NOT EXISTS transaction
(
    id               UUID        NOT NULL PRIMARY KEY DEFAULT uuid_generate_v4(),
    card_id          UUID        NOT NULL REFERENCES card (id),
    request_id       UUID        NOT NULL UNIQUE,
    authorization_id UUID        NOT NULL,
    amount           BIGINT      NOT NULL,
    currency         VARCHAR(4)  NOT NULL,
    created_date     TIMESTAMPTZ NOT NULL,
    updated_date     TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS payment_action
(
    id             UUID                  NOT NULL PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID                  NOT NULL REFERENCES transaction (id),
    type           payment_action_type   NOT NULL,
    status         payment_action_status NOT NULL,
    amount         BIGINT,
    currency       VARCHAR(4),
    request_id     UUID                  NOT NULL UNIQUE,
    created_date   TIMESTAMPTZ           NOT NULL,
    updated_date   TIMESTAMPTZ           NOT NULL
);