package generator

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"synthgraph/internal/schema"
)

type urlGenerator struct{}

func (urlGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	word := urlWords[rng.IntN(len(urlWords))]
	return fmt.Sprintf("https://www.%s.com/%s", word, randomString(8, rng)), nil
}

type codeGenerator struct{}

func (codeGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	letters := randomString(3, rng)
	digits := rng.IntN(10000)
	return strings.ToUpper(letters) + fmt.Sprintf("%04d", digits), nil
}

type slugGenerator struct{}

func (slugGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	count := rng.IntN(3) + 1
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		parts[i] = slugWords[rng.IntN(len(slugWords))]
	}
	return strings.Join(parts, "-"), nil
}
