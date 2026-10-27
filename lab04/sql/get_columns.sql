SELECT column_name, data_type
FROM information_schema.columns
WHERE table_schema = CURRENT_SCHEMA AND table_name = $1
ORDER BY ordinal_position;
