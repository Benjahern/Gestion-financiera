CREATE TABLE annual_balances (
    user_id smallint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    currency text NOT NULL CHECK (currency IN ('CLP', 'USD')),
    year smallint NOT NULL CHECK (year BETWEEN 1900 AND 9998),
    balance numeric(20,2) NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, currency, year),
    CHECK (currency <> 'CLP' OR balance = trunc(balance))
);
