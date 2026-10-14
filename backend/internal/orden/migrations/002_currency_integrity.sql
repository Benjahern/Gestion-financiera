CREATE OR REPLACE FUNCTION validate_movement_currency() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE unit text;
BEGIN
    SELECT currency INTO unit FROM accounts WHERE id=NEW.account_id AND user_id=NEW.user_id FOR UPDATE;
    IF unit='CLP' AND NEW.amount != trunc(NEW.amount) THEN
        RAISE EXCEPTION 'CLP requires whole units' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER movement_currency_check BEFORE INSERT OR UPDATE ON movements
FOR EACH ROW EXECUTE FUNCTION validate_movement_currency();

CREATE OR REPLACE FUNCTION prevent_account_currency_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.currency != OLD.currency AND EXISTS(SELECT 1 FROM movements WHERE account_id=OLD.id) THEN
        RAISE EXCEPTION 'Currency is immutable for accounts with movements' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER account_currency_check BEFORE UPDATE ON accounts
FOR EACH ROW EXECUTE FUNCTION prevent_account_currency_change();
