ALTER TABLE accounts ADD COLUMN show_in_summary boolean NOT NULL DEFAULT false;
CREATE UNIQUE INDEX accounts_summary_currency_key ON accounts(user_id,currency) WHERE show_in_summary;
