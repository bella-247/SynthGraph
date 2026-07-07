package generator

import (
	"fmt"
	"math/rand/v2"

	"synthgraph/internal/schema"
)

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

type floatGenerator struct{}

func (generator floatGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	value := float64(rng.Int64N(1000000)) / 100.0
	return value, nil
}

func pow10int64(n int) int64 {
	result := int64(1)
	for i := 0; i < n && i < 18; i++ {
		result *= 10
	}
	return result
}
