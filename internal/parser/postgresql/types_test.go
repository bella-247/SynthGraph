package postgresql

import (
	"testing"
)

func TestNormalizeType_KnownTypes(t *testing.T) {
	tests := []struct {
		input string
		want  AbstractType
	}{
		// Integer family
		{"int", TypeInt},
		{"integer", TypeInt},
		{"int4", TypeInt},
		{"int8", TypeBigInt},
		{"bigint", TypeBigInt},
		{"smallint", TypeSmallInt},
		{"int2", TypeSmallInt},

		// Text family
		{"text", TypeText},
		{"varchar", TypeVarChar},
		{"character varying", TypeVarChar},
		{"char", TypeChar},
		{"character", TypeChar},
		{"bpchar", TypeChar},

		// Boolean
		{"boolean", TypeBoolean},
		{"bool", TypeBoolean},

		// Decimal / numeric
		{"decimal", TypeDecimal},
		{"numeric", TypeDecimal},
		{"dec", TypeDecimal},

		// Float
		{"real", TypeFloat},
		{"float4", TypeFloat},
		{"float", TypeFloat},
		{"float8", TypeDouble},
		{"double precision", TypeDouble},

		// Temporal
		{"date", TypeDate},
		{"time", TypeTime},
		{"timestamp", TypeTimestamp},
		{"timestamptz", TypeTimestamp},
		{"interval", TypeInterval},

		// JSON / structured
		{"json", TypeJSON},
		{"jsonb", TypeJSONB},
		{"uuid", TypeUUID},
		{"bytea", TypeBytea},

		// Text aliases (common in pg)
		{"cidr", TypeText},
		{"citext", TypeText},
		{"name", TypeText},

		// Network types
		{"inet", TypeInet},
		{"macaddr", TypeMacAddr},

		// Unknown types (not in the type map → TypeUnknown)
		{"mood", TypeUnknown},
		{"order_status", TypeUnknown},
		{"custom_type", TypeUnknown},
	}

	for _, tt := range tests {
		got := NormalizeType(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeType_CaseInsensitive(t *testing.T) {
	inputs := []string{"INT", "Integer", "BIGINT", "Text", "VARCHAR", "BOOLEAN"}
	expected := []AbstractType{TypeInt, TypeInt, TypeBigInt, TypeText, TypeVarChar, TypeBoolean}

	for i, input := range inputs {
		got := NormalizeType(input)
		if got != expected[i] {
			t.Errorf("NormalizeType(%q) = %v, want %v", input, got, expected[i])
		}
	}
}

func TestNormalizeType_Empty(t *testing.T) {
	got := NormalizeType("")
	if got != TypeUnknown {
		t.Errorf("NormalizeType('') = %v, want %v", got, TypeUnknown)
	}
}

func TestIsSerialType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"serial", true},
		{"serial4", true},
		{"serial2", true},
		{"bigserial", true},
		{"serial8", true},
		{"smallserial", true},
		{"int", false},
		{"bigint", false},
		{"text", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsSerialType(tt.input)
		if got != tt.want {
			t.Errorf("IsSerialType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsSerialType_CaseInsensitive(t *testing.T) {
	if !IsSerialType("SERIAL") {
		t.Error("IsSerialType('SERIAL') = false, want true")
	}
	if !IsSerialType("BIGSERIAL") {
		t.Error("IsSerialType('BIGSERIAL') = false, want true")
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		abstractType AbstractType
		want         bool
	}{
		{TypeInt, true},
		{TypeBigInt, true},
		{TypeSmallInt, true},
		{TypeDecimal, true},
		{TypeFloat, true},
		{TypeDouble, true},
		{TypeText, false},
		{TypeVarChar, false},
		{TypeBoolean, false},
		{TypeTimestamp, false},
		{TypeJSON, false},
		{TypeUnknown, false},
	}

	for _, tt := range tests {
		got := IsNumeric(tt.abstractType)
		if got != tt.want {
			t.Errorf("IsNumeric(%v) = %v, want %v", tt.abstractType, got, tt.want)
		}
	}
}

func TestIsText(t *testing.T) {
	tests := []struct {
		abstractType AbstractType
		want         bool
	}{
		{TypeText, true},
		{TypeVarChar, true},
		{TypeChar, true},
		{TypeInt, false},
		{TypeBoolean, false},
		{TypeUnknown, false},
	}

	for _, tt := range tests {
		got := IsText(tt.abstractType)
		if got != tt.want {
			t.Errorf("IsText(%v) = %v, want %v", tt.abstractType, got, tt.want)
		}
	}
}

func TestIsTemporal(t *testing.T) {
	tests := []struct {
		abstractType AbstractType
		want         bool
	}{
		{TypeDate, true},
		{TypeTime, true},
		{TypeTimestamp, true},
		{TypeInterval, true},
		{TypeInt, false},
		{TypeText, false},
		{TypeEnum, false},
	}

	for _, tt := range tests {
		got := IsTemporal(tt.abstractType)
		if got != tt.want {
			t.Errorf("IsTemporal(%v) = %v, want %v", tt.abstractType, got, tt.want)
		}
	}
}
