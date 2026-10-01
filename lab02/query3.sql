SELECT last_name, first_name, middle_name, date_start
FROM lab2_employee_ilinov
WHERE extract(days from now() - date_start) / 365 > 4
ORDER BY last_name, middle_name, first_name;