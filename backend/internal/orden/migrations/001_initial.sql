CREATE TABLE IF NOT EXISTS users (
    id smallint PRIMARY KEY CHECK (id = 1),
    email text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS activation_codes (
    purpose text PRIMARY KEY CHECK (purpose IN ('activate', 'recover')),
    token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
    token_hash bytea PRIMARY KEY,
    user_id smallint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expires_at);
CREATE TABLE IF NOT EXISTS rate_limits (
    key text PRIMARY KEY,
    window_start timestamptz NOT NULL,
    attempts integer NOT NULL
);
CREATE TABLE IF NOT EXISTS accounts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id smallint NOT NULL REFERENCES users(id),
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    type text NOT NULL CHECK (length(type) BETWEEN 1 AND 100),
    currency text NOT NULL CHECK (currency IN ('CLP', 'USD')),
    opening_balance numeric(18,2) NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (currency != 'CLP' OR opening_balance = trunc(opening_balance)),
    UNIQUE (id, user_id),
    UNIQUE (user_id, name)
);
CREATE TABLE IF NOT EXISTS movements (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id smallint NOT NULL REFERENCES users(id),
    account_id bigint NOT NULL,
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 150),
    category text NOT NULL CHECK (length(category) BETWEEN 1 AND 100),
    date date NOT NULL,
    amount numeric(18,2) NOT NULL CHECK (amount > 0),
    kind text NOT NULL CHECK (kind IN ('expense', 'income', 'investment')),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (account_id, user_id) REFERENCES accounts(id, user_id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS movements_user_date_idx ON movements(user_id, date DESC, id DESC);
CREATE INDEX IF NOT EXISTS movements_account_idx ON movements(account_id);
