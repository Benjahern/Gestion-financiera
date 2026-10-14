ALTER TABLE movements ADD COLUMN pending boolean NOT NULL DEFAULT false;
ALTER TABLE movements ADD COLUMN category_confirmed boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN categories_initialized boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN gmail_generation bigint NOT NULL DEFAULT 0;
CREATE TABLE expense_categories (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id smallint NOT NULL REFERENCES users(id),
 name text NOT NULL CHECK(length(name) BETWEEN 1 AND 100),
 UNIQUE(user_id,name), UNIQUE(id,user_id)
);
CREATE TABLE training_examples (
 movement_id bigint PRIMARY KEY REFERENCES movements(id) ON DELETE CASCADE,
 user_id smallint NOT NULL REFERENCES users(id),
 description text NOT NULL,
 category_id bigint NOT NULL,
 FOREIGN KEY(category_id,user_id) REFERENCES expense_categories(id,user_id) ON DELETE CASCADE
);
CREATE TABLE gmail_connections (
 user_id smallint PRIMARY KEY REFERENCES users(id),
 email text NOT NULL,
 credentials bytea NOT NULL,
 connected_at timestamptz NOT NULL DEFAULT now(),
 last_sync timestamptz,
 page_token text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'connected',
 last_error text NOT NULL DEFAULT ''
);
CREATE TABLE gmail_oauth_states (
 state_hash bytea PRIMARY KEY,
 user_id smallint NOT NULL REFERENCES users(id),
 session_hash bytea NOT NULL REFERENCES sessions(token_hash) ON DELETE CASCADE,
 verifier text NOT NULL,
 generation bigint NOT NULL,
 expires_at timestamptz NOT NULL
);
CREATE TABLE gmail_card_rules (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id smallint NOT NULL REFERENCES users(id),
 bank text NOT NULL CHECK(bank IN ('mach','bci')),
 last_four text NOT NULL CHECK(last_four ~ '^[0-9]{4}$'),
 account_id bigint NOT NULL,
 UNIQUE(user_id,bank,last_four),
 FOREIGN KEY(account_id,user_id) REFERENCES accounts(id,user_id) ON DELETE CASCADE
);
CREATE TABLE gmail_imports (
 user_id smallint NOT NULL REFERENCES users(id),
 message_id text NOT NULL,
 purchase_id text NOT NULL,
 movement_id bigint REFERENCES movements(id) ON DELETE SET NULL,
 PRIMARY KEY(user_id,message_id),
 UNIQUE(user_id,purchase_id)
);
