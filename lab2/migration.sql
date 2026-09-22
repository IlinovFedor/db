CREATE TABLE lab2_employee_ilinov (
    id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    first_name TEXT NOT NULL,
    middle_name TEXT,
    last_name TEXT NOT NULL,
    salary INTEGER NOT NULL,
    address TEXT NOT NULL,
    phone_number TEXT NOT NULL,
    date_start TIMESTAMPTZ NOT NULL
);