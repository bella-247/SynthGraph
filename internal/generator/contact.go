package generator

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"synthgraph/internal/schema"
)

type emailGenerator struct{}

func (emailGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	name := strings.ToLower(firstNames[rng.IntN(len(firstNames))])
	domain := emailDomains[rng.IntN(len(emailDomains))]
	return fmt.Sprintf("%s%d@%s", name, rng.IntN(999)+1, domain), nil
}

type phoneGenerator struct{}

func (phoneGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	area := rng.IntN(800) + 200
	prefix := rng.IntN(1000)
	line := rng.IntN(10000)
	return fmt.Sprintf("(%d) %03d-%04d", area, prefix, line), nil
}

type addressGenerator struct{}

func (addressGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	number := rng.IntN(9999) + 1
	street := streetNames[rng.IntN(len(streetNames))]
	return fmt.Sprintf("%d %s", number, street), nil
}

type zipGenerator struct{}

func (zipGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	return fmt.Sprintf("%05d", rng.IntN(90000)+10000), nil
}
