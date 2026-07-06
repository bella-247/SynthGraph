package postgresql

import "strings"

// AbstractType is SynthGraph's canonical type classification.
type AbstractType string

const (
	TypeInt       AbstractType = "int"
	TypeBigInt    AbstractType = "bigint"
	TypeSmallInt  AbstractType = "smallint"
	TypeText      AbstractType = "text"
	TypeVarChar   AbstractType = "varchar"
	TypeChar      AbstractType = "char"
	TypeBoolean   AbstractType = "boolean"
	TypeDecimal   AbstractType = "decimal"
	TypeFloat     AbstractType = "float"
	TypeDouble    AbstractType = "double"
	TypeDate      AbstractType = "date"
	TypeTime      AbstractType = "time"
	TypeTimestamp AbstractType = "timestamp"
	TypeUUID      AbstractType = "uuid"
	TypeJSON      AbstractType = "json"
	TypeJSONB     AbstractType = "jsonb"
	TypeBytea     AbstractType = "bytea"
	TypeInterval  AbstractType = "interval"
	TypeEnum      AbstractType = "enum"
	TypeInet      AbstractType = "inet"
	TypeMacAddr   AbstractType = "macaddr"
	TypeUnknown   AbstractType = "unknown"
)

var typeMap = map[string]AbstractType{
	"int":                TypeInt,
	"integer":            TypeInt,
	"int4":               TypeInt,
	"serial":             TypeInt,
	"serial4":            TypeInt,
	"bigint":             TypeBigInt,
	"int8":               TypeBigInt,
	"bigserial":          TypeBigInt,
	"serial8":            TypeBigInt,
	"smallint":           TypeSmallInt,
	"int2":               TypeSmallInt,
	"smallserial":        TypeSmallInt,
	"serial2":            TypeSmallInt,
	"text":               TypeText,
	"varchar":            TypeVarChar,
	"character varying":  TypeVarChar,
	"char":               TypeChar,
	"character":          TypeChar,
	"bpchar":             TypeChar,
	"name":               TypeText,
	"citext":             TypeText,
	"bool":               TypeBoolean,
	"boolean":            TypeBoolean,
	"numeric":            TypeDecimal,
	"decimal":            TypeDecimal,
	"dec":                TypeDecimal,
	"real":               TypeFloat,
	"float4":             TypeFloat,
	"float":              TypeFloat,
	"double":             TypeDouble,
	"double precision":   TypeDouble,
	"float8":             TypeDouble,
	"date":               TypeDate,
	"time":               TypeTime,
	"timetz":             TypeTime,
	"time with time zone":            TypeTime,
	"time without time zone":         TypeTime,
	"timestamp":          TypeTimestamp,
	"timestamptz":        TypeTimestamp,
	"timestamp with time zone":       TypeTimestamp,
	"timestamp without time zone":    TypeTimestamp,
	"interval":           TypeInterval,
	"uuid":               TypeUUID,
	"json":               TypeJSON,
	"jsonb":              TypeJSONB,
	"bytea":              TypeBytea,
	"inet":               TypeInet,
	"cidr":               TypeText,
	"macaddr":            TypeMacAddr,
}

// IsSerialType returns true if the PostgreSQL type name is a SERIAL variant.
func IsSerialType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "serial", "serial4", "serial2",
		"bigserial", "serial8",
		"smallserial":
		return true
	}
	return false
}

// Normalize takes a raw PostgreSQL type name and returns the abstract type.
// Returns TypeUnknown for unrecognized types.
func NormalizeType(raw string) AbstractType {
	if t, ok := typeMap[strings.ToLower(raw)]; ok {
		return t
	}
	return TypeUnknown
}

// IsNumeric returns true for numeric abstract types.
func IsNumeric(t AbstractType) bool {
	switch t {
	case TypeInt, TypeBigInt, TypeSmallInt, TypeDecimal, TypeFloat, TypeDouble:
		return true
	}
	return false
}

// IsText returns true for text abstract types.
func IsText(t AbstractType) bool {
	switch t {
	case TypeText, TypeVarChar, TypeChar:
		return true
	}
	return false
}

// IsTemporal returns true for date/time abstract types.
func IsTemporal(t AbstractType) bool {
	switch t {
	case TypeDate, TypeTime, TypeTimestamp, TypeInterval:
		return true
	}
	return false
}
