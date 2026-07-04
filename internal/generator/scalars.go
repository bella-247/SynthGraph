package generator

import (
	"fmt"
	"math"
	"math/rand/v2"

	"synthgraph/internal/schema"
)

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
		maxLen = 50
	}
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

// boolGenerator generates random boolean values.
type boolGenerator struct{}

func (generator boolGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	return rng.IntN(2) == 0, nil
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
