ALTER TABLE movements
    ADD COLUMN investment_mode text NOT NULL DEFAULT 'legacy_outflow'
        CHECK (investment_mode IN ('legacy_outflow', 'contribution')),
    ADD COLUMN source_account_id bigint;

ALTER TABLE movements
    ADD CONSTRAINT movements_source_account_owner_fk
        FOREIGN KEY (source_account_id, user_id) REFERENCES accounts(id, user_id) ON DELETE RESTRICT,
    ADD CONSTRAINT movements_investment_source_distinct
        CHECK (source_account_id IS NULL OR source_account_id <> account_id),
    ADD CONSTRAINT movements_investment_source_mode
        CHECK (source_account_id IS NULL OR (kind = 'investment' AND investment_mode = 'contribution'));

CREATE INDEX movements_source_account_idx ON movements(source_account_id) WHERE source_account_id IS NOT NULL;

CREATE OR REPLACE FUNCTION validate_movement_currency() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    destination_currency text;
    source_currency text;
    destination_investment boolean;
BEGIN
    SELECT currency, is_investment INTO destination_currency, destination_investment
    FROM accounts WHERE id=NEW.account_id AND user_id=NEW.user_id FOR UPDATE;
    IF destination_currency IS NULL THEN
        RAISE EXCEPTION 'Unknown destination account' USING ERRCODE='23503';
    END IF;
    IF destination_currency='CLP' AND NEW.amount != trunc(NEW.amount) THEN
        RAISE EXCEPTION 'CLP requires whole units' USING ERRCODE='23514';
    END IF;
    IF NEW.investment_mode='contribution' THEN
        IF NEW.kind <> 'investment' OR NOT destination_investment THEN
            RAISE EXCEPTION 'Investment contributions require an investment destination' USING ERRCODE='23514';
        END IF;
        IF NEW.source_account_id IS NOT NULL THEN
            SELECT currency INTO source_currency FROM accounts
            WHERE id=NEW.source_account_id AND user_id=NEW.user_id FOR UPDATE;
            IF source_currency IS NULL OR source_currency <> destination_currency THEN
                RAISE EXCEPTION 'Investment source currency must match destination' USING ERRCODE='23514';
            END IF;
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION prevent_account_currency_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.currency != OLD.currency AND (
        EXISTS(SELECT 1 FROM movements WHERE account_id=OLD.id)
        OR EXISTS(SELECT 1 FROM movements WHERE source_account_id=OLD.id)
    ) THEN
        RAISE EXCEPTION 'Currency is immutable for accounts with movements' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
