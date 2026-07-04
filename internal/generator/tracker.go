package generator

import "synthgraph/internal/schema"

// uniqueTracker ensures generated values for UNIQUE columns don't repeat.
type uniqueTracker struct {
	uniqueCols map[string]bool
	seen       map[string]map[any]bool
}

func newUniqueTracker(table *schema.Table) *uniqueTracker {
	tracker := &uniqueTracker{
		uniqueCols: make(map[string]bool),
		seen:       make(map[string]map[any]bool),
	}

	for _, uniqueConstraint := range table.Unique {
		for _, col := range uniqueConstraint {
			tracker.uniqueCols[col] = true
		}
	}

	return tracker
}

func (tracker *uniqueTracker) isUniqueColumn(name string) bool {
	return tracker.uniqueCols[name]
}

func (tracker *uniqueTracker) checkSeen(column string, value any) bool {
	if tracker.seen[column] == nil {
		return false
	}
	return tracker.seen[column][value]
}

func (tracker *uniqueTracker) record(column string, value any) {
	if tracker.seen[column] == nil {
		tracker.seen[column] = make(map[any]bool)
	}
	tracker.seen[column][value] = true
}

func isPrimaryKeyColumn(columnName string, pk []string) bool {
	for _, pkCol := range pk {
		if pkCol == columnName {
			return true
		}
	}
	return false
}
