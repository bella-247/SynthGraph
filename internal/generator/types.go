package generator

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"

	"synthgraph/internal/schema"
)

// ── Type generator registry ───────────────────────────────────────────────

// typeGeneratorFor returns the appropriate TypeGenerator for a column type.
// For built-in types, it returns the corresponding standard generator.
// For enum types (not in the built-in set), it returns an enum generator.
func typeGeneratorFor(columnType string, model *schema.Model, enumValues map[string][]string) TypeGenerator {
	if generator, isBuiltIn := builtInGenerators[columnType]; isBuiltIn {
		return generator
	}

	// Check if this is an enum type.
	if values, isEnum := enumValues[columnType]; isEnum {
		return enumGenerator(values)
	}

	return unknownTypeGenerator(columnType)
}

// builtInGenerators maps canonical type names to their generators.
var builtInGenerators = map[string]TypeGenerator{
	"int":      intGenerator{min: 1, max: 2147483647},
	"int4":     intGenerator{min: 1, max: 2147483647},
	"int8":     intGenerator{min: 1, max: 9223372036854775807},
	"bigint":   intGenerator{min: 1, max: 9223372036854775807},
	"smallint": intGenerator{min: 1, max: 32767},
	"serial":   intGenerator{min: 1, max: 2147483647},
	"bigserial": intGenerator{min: 1, max: 9223372036854775807},
	"smallserial": intGenerator{min: 1, max: 32767},

	"varchar":  stringGenerator{},
	"text":     stringGenerator{},
	"char":     stringGenerator{},

	"uuid": uuidGenerator{},

	"timestamp":   timestampGenerator{},
	"timestamptz": timestampGenerator{},
	"date":        timestampGenerator{},
	"time":        timestampGenerator{},
	"timetz":      timestampGenerator{},

	"boolean": boolGenerator{},

	"decimal": decimalGenerator{},
	"numeric": decimalGenerator{},
	"float4":  floatGenerator{},
	"float8":  floatGenerator{},
	"real":    floatGenerator{},
	"double precision": floatGenerator{},

	"json":  jsonGenerator{},
	"jsonb": jsonGenerator{},

	"inet":    inetGenerator{},
	"macaddr": macaddrGenerator{},
}

// ── Individual generators ─────────────────────────────────────────────────

// intGenerator generates random integer values.
type intGenerator struct {
	min int64
	max int64
}

func (generator intGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	if generator.max <= generator.min {
		return generator.min, nil
	}
	rangeSize := generator.max - generator.min + 1
	if rangeSize > math.MaxInt64 {
		// For very large ranges (unlikely in practice), shift down.
		value := generator.min + int64(rng.Uint64()%uint64(rangeSize))
		return value, nil
	}
	value := generator.min + rng.Int64N(rangeSize)
	return value, nil
}

// stringGenerator generates random string values.
type stringGenerator struct{}

func (generator stringGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	maxLen := column.Length
	if maxLen <= 0 {
		maxLen = 50 // sensible default for unbounded text
	}

	// Pick a random length between 1 and maxLen.
	length := rng.IntN(maxLen) + 1
	if length < 1 {
		length = 1
	}

	return randomString(length, rng), nil
}

// uuidGenerator generates random UUID v4 strings.
type uuidGenerator struct{}

func (generator uuidGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	return generateUUID(rng), nil
}

// timestampGenerator generates random timestamp strings.
type timestampGenerator struct{}

func (generator timestampGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	// Generate a timestamp within a reasonable range (2020-01-01 to 2026-12-31).
	// Use Unix timestamps for simplicity.
	minTime := int64(1577836800) // 2020-01-01
	maxTime := int64(1798761600) // 2026-12-31
	rangeSize := maxTime - minTime + 1

	unixTime := minTime + rng.Int64N(rangeSize)

	return formatTimestamp(unixTime), nil
}

// boolGenerator generates random boolean values.
type boolGenerator struct{}

func (generator boolGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	return rng.IntN(2) == 0, nil
}

// decimalGenerator generates random decimal values as strings.
type decimalGenerator struct{}

func (generator decimalGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	precision := column.Precision
	if precision <= 0 {
		precision = 2
	}
	length := column.Length
	if length <= 0 {
		length = 10
	}

	integerPart := length - precision
	if integerPart < 1 {
		integerPart = 1
	}

	// Generate integer part: 1 to 10^integerPart - 1.
	intMax := pow10int64(integerPart)
	intValue := rng.Int64N(intMax-1) + 1

	// Generate fractional part.
	fracValue := rng.Int64N(pow10int64(precision))

	return fmt.Sprintf("%d.%0*d", intValue, precision, fracValue), nil
}

// floatGenerator generates random float values.
type floatGenerator struct{}

func (generator floatGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	// Generate a random float between 0 and 10000 with 2 decimal places.
	value := float64(rng.Int64N(1000000)) / 100.0
	return value, nil
}

// jsonGenerator generates simple JSON objects.
type jsonGenerator struct{}

func (generator jsonGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	// Generate a simple JSON object with a few random fields.
	fieldCount := rng.IntN(4) + 1
	pairs := make([]string, 0, fieldCount)
	for i := 0; i < fieldCount; i++ {
		key := randomString(8, rng)
		value := rng.Int64N(1000)
		pairs = append(pairs, fmt.Sprintf(`"%s":%d`, key, value))
	}
	return `{` + strings.Join(pairs, ",") + `}`, nil
}

// inetGenerator generates random IP address strings.
type inetGenerator struct{}

func (generator inetGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	return fmt.Sprintf("%d.%d.%d.%d",
		rng.IntN(223)+1,   // 1-223 (avoid multicast)
		rng.IntN(256),
		rng.IntN(256),
		rng.IntN(254)+1,   // 1-254 (avoid .0 and .255)
	), nil
}

// macaddrGenerator generates random MAC address strings.
type macaddrGenerator struct{}

func (generator macaddrGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	parts := make([]string, 6)
	for i := range parts {
		parts[i] = fmt.Sprintf("%02x", rng.IntN(256))
	}
	return strings.Join(parts, ":"), nil
}

// enumGenerator returns a TypeGenerator that picks from a fixed set of values.
func enumGenerator(values []string) TypeGenerator {
	return TypeGeneratorFunc(func(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
		if len(values) == 0 {
			return "", nil
		}
		return values[rng.IntN(len(values))], nil
	})
}

// unknownTypeGenerator returns a generator for types we don't explicitly handle.
func unknownTypeGenerator(typeName string) TypeGenerator {
	return TypeGeneratorFunc(func(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
		// For unknown types, generate a placeholder string.
		length := column.Length
		if length <= 0 {
			length = 20
		}
		if length > 100 {
			length = 100
		}
		return fmt.Sprintf("<%s:%s>", typeName, randomString(length, rng)), nil
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────

// randomString generates a random alphanumeric string of the given length.
func randomString(length int, rng *rand.Rand) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 _-.',"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[rng.IntN(len(chars))]
	}
	return string(result)
}

// formatTimestamp formats a Unix timestamp as an ISO 8601 string.
func formatTimestamp(unixTime int64) string {
	// Simple implementation without time.Time (avoids timezone complications).
	// Returns a string like "2024-03-15 14:30:00" in UTC.
	secondsInDay := int64(86400)
	days := unixTime / secondsInDay
	timeOfDay := unixTime % secondsInDay

	hours := timeOfDay / 3600
	minutes := (timeOfDay % 3600) / 60
	seconds := timeOfDay % 60

	// Approximate date from days since 2020-01-01.
	year, month, day := daysToDate(days)

	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", year, month, day, hours, minutes, seconds)
}

// daysToDate converts a day count (since 2020-01-01) to year/month/day.
// Uses a simple calendar with 30-day months for determinism (not astronomically precise).
func daysToDate(days int64) (int, int, int) {
	// Start from 2020-01-01.
	daysInMonth := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	isLeap := func(year int) bool {
		return year%4 == 0 && (year%100 != 0 || year%400 == 0)
	}

	year := 2020
	remaining := days

	for {
		yearDays := 365
		if isLeap(year) {
			yearDays = 366
		}
		if remaining < int64(yearDays) {
			break
		}
		remaining -= int64(yearDays)
		year++
	}

	dim := make([]int, 12)
	copy(dim, daysInMonth)
	if isLeap(year) {
		dim[1] = 29
	}

	month := 1
	for ; month <= 12; month++ {
		if remaining < int64(dim[month-1]) {
			break
		}
		remaining -= int64(dim[month-1])
	}

	day := int(remaining) + 1
	if month > 12 {
		month = 12
		day = 31
	}
	if day < 1 {
		day = 1
	}

	return year, month, day
}

// pow10int64 returns 10^n for n >= 0, clamped to avoid overflow.
func pow10int64(n int) int64 {
	result := int64(1)
	for i := 0; i < n && i < 18; i++ {
		result *= 10
	}
	return result
}
