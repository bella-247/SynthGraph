package generator

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"synthgraph/internal/schema"
)

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

type inetGenerator struct{}

func (generator inetGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	return fmt.Sprintf("%d.%d.%d.%d",
		rng.IntN(223)+1,
		rng.IntN(256),
		rng.IntN(256),
		rng.IntN(254)+1,
	), nil
}

type macaddrGenerator struct{}

func (generator macaddrGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	parts := make([]string, 6)
	for i := range parts {
		parts[i] = fmt.Sprintf("%02x", rng.IntN(256))
	}
	return strings.Join(parts, ":"), nil
}
