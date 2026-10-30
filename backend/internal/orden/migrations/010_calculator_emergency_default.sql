DO $$
BEGIN
    IF to_regclass('calculator_settings') IS NOT NULL THEN
        EXECUTE $update$
            UPDATE calculator_settings
            SET settings = jsonb_set(settings, '{emergency_goal}', '"300000"'::jsonb, true),
                updated_at = now()
            WHERE currency = 'CLP'
        $update$;
    END IF;
END;
$$;
