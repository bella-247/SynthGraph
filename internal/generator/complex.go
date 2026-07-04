package generator

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"synthgraph/internal/schema"
)

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

	intMax := pow10int64(integerPart)
	intValue := rng.Int64N(intMax-1) + 1
	fracValue := rng.Int64N(pow10int64(precision))

	return fmt.Sprintf("%d.%0*d", intValue, precision, fracValue), nil
}

// floatGenerator generates random float values.
type floatGenerator struct{}

func (generator floatGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	value := float64(rng.Int64N(1000000)) / 100.0
	return value, nil
}

// jsonGenerator generates simple JSON objects.
type jsonGenerator struct{}

func (generator jsonGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
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
		rng.IntN(223)+1,
		rng.IntN(256),
		rng.IntN(256),
		rng.IntN(254)+1,
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

// pow10int64 returns 10^n for n >= 0, clamped to avoid overflow.
func pow10int64(n int) int64 {
	result := int64(1)
	for i := 0; i < n && i < 18; i++ {
		result *= 10
	}
	return result
}
