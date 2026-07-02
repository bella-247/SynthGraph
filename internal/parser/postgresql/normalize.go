package postgresql

import (
	"strings"

	"synthgraph/internal/schema"
)

// normalize runs the second pipeline stage: canonicalization.
//
// Responsibilities:
//   - Map PostgreSQL types to SynthGraph abstract types
//     (e.g. INT4/INTEGER/INT → int, CHARACTER VARYING → varchar).
//   - Resolve SERIAL variants to concrete integer types
//     (SERIAL → int, BIGSERIAL → bigint, SMALLSERIAL → smallint).
//   - Treat unrecognised types as enum references (preserving the raw name).
//   - Set nullability from NOT NULL / nullable default.
//   - Store DEFAULT expressions as raw strings.
//
// Contract:
//   - Equivalent SQL syntax across dialects produces identical normalised output.
//   - Every column has a populated schema.Column with canonical type.
//   - No cross-reference resolution happens here (that is the link stage).
//   - No consistency checking happens here (that is the validate stage).
func (translator *schemaTranslator) normalize() {
	for tableIndex := range translator.tables {
		table := &translator.tables[tableIndex]
		table.columns = make([]schema.Column, len(table.rawColumns))

		for columnIndex, raw := range table.rawColumns {
			table.columns[columnIndex] = normalizeColumn(raw)
		}
	}
}

// normalizeColumn converts a raw ColumnDef into a canonical schema.Column.
func normalizeColumn(raw ColumnDef) schema.Column {
	column := schema.Column{
		Name: raw.Name,
	}

	baseType := strings.ToLower(raw.Type.BaseType)
	abstractType := NormalizeType(baseType)

	// Handle SERIAL types: SERIAL4 → INT, SERIAL8 → BIGINT
	if raw.Type.IsSerial || IsSerialType(baseType) {
		switch baseType {
		case "bigserial", "serial8":
			abstractType = TypeBigInt
		case "smallserial", "serial2":
			abstractType = TypeSmallInt
		default:
			abstractType = TypeInt
		}
	}

	// Unknown type → treat as enum reference
	if abstractType == TypeUnknown {
		abstractType = TypeEnum
	}

	column.Type = string(abstractType)
	if abstractType == TypeEnum {
		// Preserve the original type name for enum lookups
		column.Type = raw.Type.BaseType
	}

	// Preserve type metadata for types that carry length/precision
	column.Length = raw.Type.Length
	column.Precision = raw.Type.Precision

	// Nullability: default is nullable unless NOT NULL or PRIMARY KEY is set
	column.Nullable = !raw.NotNull && !raw.IsPrimaryKey

	// Default: store as raw string
	if raw.Default != "" {
		defaultValue := raw.Default
		column.Default = &defaultValue
	}

	return column
}
