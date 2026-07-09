package generator

import (
	"fmt"
	"strings"

	"synthgraph/internal/schema"
)

const maxRowRetries = 100

type compositeViolation struct {
	columns  []string
	key      string
	firstRow int
}

func (v *compositeViolation) Error() string {
	return fmt.Sprintf("duplicate composite key %q for columns %v (first seen at row %d)", v.key, v.columns, v.firstRow)
}

// uniqueTracker ensures generated values for UNIQUE columns don't repeat.
// It handles both single-column and composite (multi-column) unique constraints,
// including composite primary keys.
type uniqueTracker struct {
	uniqueCols map[string]bool
	seen       map[string]map[any]bool

	rowGroups [][]string
	rowSeen   map[string]int
}

func newUniqueTracker(table *schema.Table) *uniqueTracker {
	tracker := &uniqueTracker{
		uniqueCols: make(map[string]bool),
		seen:       make(map[string]map[any]bool),
		rowSeen:    make(map[string]int),
	}

	for _, uniqueConstraint := range table.Unique {
		if len(uniqueConstraint) == 1 {
			tracker.uniqueCols[uniqueConstraint[0]] = true
		} else {
			tracker.rowGroups = append(tracker.rowGroups, uniqueConstraint)
		}
	}

	if len(table.PrimaryKey) > 1 {
		tracker.rowGroups = append(tracker.rowGroups, table.PrimaryKey)
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

// compositeGroupForColumn returns the composite constraint groups that contain
// the given column, along with the column's index within each group.
func (tracker *uniqueTracker) compositeGroupForColumn(colName string) ([]string, int, bool) {
	for _, group := range tracker.rowGroups {
		for idx, member := range group {
			if member == colName {
				return group, idx, true
			}
		}
	}
	return nil, 0, false
}

// isCompositeKeySeen checks whether a specific composite key has been recorded.
func (tracker *uniqueTracker) isCompositeKeySeen(key string) bool {
	_, seen := tracker.rowSeen[key]
	return seen
}

func (tracker *uniqueTracker) checkRowConstraints(row GeneratedRow) *compositeViolation {
	for _, group := range tracker.rowGroups {
		key := serializeRowKey(row, group)
		if firstRow, seen := tracker.rowSeen[key]; seen {
			return &compositeViolation{
				columns:  group,
				key:      key,
				firstRow: firstRow,
			}
		}
	}
	return nil
}

func (tracker *uniqueTracker) recordRowConstraints(row GeneratedRow, rowIndex int) {
	for _, group := range tracker.rowGroups {
		key := serializeRowKey(row, group)
		if _, exists := tracker.rowSeen[key]; !exists {
			tracker.rowSeen[key] = rowIndex
		}
	}
}

func serializeRowKey(row GeneratedRow, columns []string) string {
	parts := make([]string, len(columns))
	for i, col := range columns {
		parts[i] = fmt.Sprintf("%v", row[col])
	}
	return strings.Join(parts, "::")
}
