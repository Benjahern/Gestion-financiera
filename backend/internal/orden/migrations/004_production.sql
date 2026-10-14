-- Remove the old installation-specific email constraint, including on existing databases.
-- Single ownership remains enforced by users.id = 1 and OWNER_EMAIL in the API.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_check;
ALTER TABLE gmail_connections ADD COLUMN scan_after timestamptz;
ALTER TABLE gmail_connections ADD COLUMN scan_until timestamptz;
ALTER TABLE gmail_connections DROP COLUMN status;
CREATE TABLE gmail_pending_messages (
 user_id smallint NOT NULL REFERENCES users(id),
 message_id text NOT NULL,
 last_attempt timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,message_id)
);

CREATE OR REPLACE FUNCTION prevent_account_currency_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.currency != OLD.currency AND (
  EXISTS(SELECT 1 FROM movements WHERE account_id=OLD.id) OR
  EXISTS(SELECT 1 FROM gmail_card_rules WHERE account_id=OLD.id)
 ) THEN
  RAISE EXCEPTION 'Currency is immutable for linked accounts' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END;
$$;
