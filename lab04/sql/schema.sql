CREATE TABLE IF NOT EXISTS departments (
    id INTEGER PRIMARY KEY,
    dept_name TEXT NOT NULL,
    room TEXT NOT NULL,
    budget INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS lab2_employee_ilinov (
    id INTEGER PRIMARY KEY,
    first_name TEXT NOT NULL,
    middle_name TEXT,
    last_name TEXT NOT NULL,
    salary INTEGER NOT NULL,
    address TEXT NOT NULL,
    phone_number TEXT NOT NULL,
    date_start DATE NOT NULL,
    department_id INTEGER REFERENCES departments(id) ON DELETE CASCADE
);
