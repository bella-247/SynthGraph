package postgresql

import "strings"

// enumKey builds a canonical enum name, schema-qualified if applicable.
func enumKey(schema, name string) string {
	if schema != "" {
		return schema + "." + name
	}
	return name
}

// tableName builds a canonical table name, schema-qualified if applicable.
func tableName(schema, name string) string {
	if schema != "" {
		return schema + "." + name
	}
	return name
}

// dedupe removes duplicate strings while preserving order.
func dedupe(values []string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			unique = append(unique, value)
		}
	}
	return unique
}

// dedupeUniques removes unique constraints already covered by the primary key.
func dedupeUniques(uniques [][]string, primaryKey []string) [][]string {
	primaryKeySet := make(map[string]bool, len(primaryKey))
	for _, column := range primaryKey {
		primaryKeySet[column] = true
	}

	isCoveredByPrimaryKey := func(columns []string) bool {
		for _, column := range columns {
			if !primaryKeySet[column] {
				return false
			}
		}
		return len(columns) > 0
	}

	seen := make(map[string]bool)
	result := make([][]string, 0)

	for _, constraint := range uniques {
		if isCoveredByPrimaryKey(constraint) {
			continue
		}
		key := strings.Join(constraint, ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, constraint)
	}

	return result
}
