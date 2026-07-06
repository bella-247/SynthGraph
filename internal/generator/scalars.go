package generator

import (
	"fmt"
	"math"
	"math/rand/v2"

	"synthgraph/internal/schema"
)

// serialGenerator generates sequential integer values (1, 2, 3, ...).
type serialGenerator struct {
	start int64
}

func (g serialGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	return g.start + int64(rowIndex), nil
}

// intGenerator generates random integer values.
type intGenerator struct {
	min int64
	max int64
}

func (g intGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	if g.max <= g.min {
		return g.min, nil
	}
	rangeSize := g.max - g.min + 1
	if rangeSize > math.MaxInt64 {
		value := g.min + int64(rng.Uint64()%uint64(rangeSize))
		return value, nil
	}
	value := g.min + rng.Int64N(rangeSize)
	return value, nil
}

// uuidGenerator generates random UUID v4 strings.
type uuidGenerator struct{}

func (uuidGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	return generateUUID(rng), nil
}

// boolGenerator generates random boolean values.
type boolGenerator struct{}

func (boolGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	return rng.IntN(2) == 0, nil
}

// stringGenerator generates random alphanumeric strings.
// Semantic-aware generation (names, emails, etc.) is handled by semantic.go.
type stringGenerator struct{}

func (stringGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	maxLen := column.Length
	if maxLen <= 0 {
		maxLen = 50
	}
	length := rng.IntN(maxLen) + 1
	if length < 1 {
		length = 1
	}
	return randomString(length, rng), nil
}

// ── Enum and unknown generators ───────────────────────────────────────────

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
