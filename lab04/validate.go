package main

import (
	"fmt"
	"slices"
	"strings"
)

var (
	allowedColumnTypes = []string{"INTEGER", "TEXT", "BOOLEAN", "DATE"}
	allowedCompareOps  = []string{"=", "<>", "<", ">", "<=", ">=", "LIKE", "ILIKE", "IS NULL", "IS NOT NULL"}
	allowedSelectOps   = slices.Concat(allowedCompareOps, []string{"IN", "NOT IN"})
	allowedFKActions   = []string{"", "NO ACTION", "CASCADE", "SET NULL", "RESTRICT"}
)

func checkAllowed(kind, value string, allowed []string) error {
	if slices.Contains(allowed, strings.ToUpper(strings.TrimSpace(value))) {
		return nil
	}
	return fmt.Errorf("invalid %s: %q", kind, value)
}

func checkWhereOp(op string) error {
	if op == "" {
		return nil
	}
	return checkAllowed("WHERE operator", op, allowedCompareOps)
}

func checkCreateColumns(cols []ColumnDef) error {
	for _, c := range cols {
		if c.Name == "" {
			continue
		}
		if err := checkAllowed("column type", c.DataType, allowedColumnTypes); err != nil {
			return err
		}
		if err := checkAllowed("ON DELETE action", c.FKOnDelete, allowedFKActions); err != nil {
			return err
		}
		if err := checkAllowed("ON UPDATE action", c.FKOnUpdate, allowedFKActions); err != nil {
			return err
		}
	}
	return nil
}
