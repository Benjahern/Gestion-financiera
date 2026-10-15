ALTER TABLE accounts ADD COLUMN is_investment boolean NOT NULL DEFAULT false;

UPDATE accounts SET is_investment = true
WHERE lower(btrim(type)) IN ('inversión', 'inversion', 'inversiones', 'cuenta de inversión', 'cuenta de inversion');
