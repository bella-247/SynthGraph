package generator

import (
	"math/rand/v2"

	"synthgraph/internal/schema"
)

var semanticGenerators = map[schema.SemanticType]TypeGenerator{
	schema.SemanticFirstName:   firstNameGenerator{},
	schema.SemanticLastName:    lastNameGenerator{},
	schema.SemanticFullName:    fullNameGenerator{},
	schema.SemanticEmail:       emailGenerator{},
	schema.SemanticPhone:       phoneGenerator{},
	schema.SemanticAddress:     addressGenerator{},
	schema.SemanticCity:        pickerGenerator{values: cities},
	schema.SemanticCountry:     pickerGenerator{values: countries},
	schema.SemanticState:       pickerGenerator{values: states},
	schema.SemanticZip:         zipGenerator{},
	schema.SemanticURL:         urlGenerator{},
	schema.SemanticDescription: sentenceGenerator{},
	schema.SemanticTitle:       titleGenerator{},
	schema.SemanticStatus:      pickerGenerator{values: statuses},
	schema.SemanticColor:       pickerGenerator{values: colors},
	schema.SemanticCategory:    pickerGenerator{values: categories},
	schema.SemanticCode:        codeGenerator{},
	schema.SemanticSlug:        slugGenerator{},
}

func semanticGeneratorFor(semantic schema.SemanticType) (TypeGenerator, bool) {
	generator, ok := semanticGenerators[semantic]
	return generator, ok
}

type pickerGenerator struct {
	values []string
}

func (g pickerGenerator) Generate(_ *schema.Column, _ int, rng *rand.Rand) (any, error) {
	if len(g.values) == 0 {
		return "", nil
	}
	return g.values[rng.IntN(len(g.values))], nil
}
