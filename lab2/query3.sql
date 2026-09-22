SELECT last_name, middle_name, first_name, date_start
FROM lab2_employee_ilinov
WHERE extract(YEAR FROM now()) - extract(YEAR FROM date_start) > 4
ORDER BY last_name, middle_name, first_name;