ALTER TABLE {{quote .Table}} ALTER COLUMN {{quote .Column}} TYPE {{.Type}} USING {{quote .Column}}::{{.Type}};
