-- Private planning preferences only. Never changes balances or movements.
CREATE TABLE calculator_settings (
    user_id smallint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    currency text NOT NULL CHECK (currency IN ('CLP', 'USD')),
    settings jsonb NOT NULL CHECK (jsonb_typeof(settings) = 'object'),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, currency)
);
