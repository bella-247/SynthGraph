package generator

import (
	"math/rand/v2"

	"synthgraph/internal/schema"
)

type firstNameGenerator struct{}

func (firstNameGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	return firstNames[rng.IntN(len(firstNames))], nil
}

type lastNameGenerator struct{}

func (lastNameGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	return lastNames[rng.IntN(len(lastNames))], nil
}

type fullNameGenerator struct{}

func (fullNameGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	return firstNames[rng.IntN(len(firstNames))] + " " + lastNames[rng.IntN(len(lastNames))], nil
}
