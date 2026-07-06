package generator

import (
	"math/rand/v2"

	"synthgraph/internal/schema"
)

// Registry is a pluggable, composable map of semantic types to generators.
// Use NewRegistry() to create one, Register() to add or override generators,
// and GeneratorFor() to look up a generator for a given semantic type.
//
// The package provides a defaultRegistry with all built-in generators.
// Callers can supply a custom Registry via GenerationContext.Registry to
// override or extend the built-in set without modifying global state.
type Registry struct {
	generators map[schema.SemanticType]TypeGenerator
}

// NewRegistry creates and returns an empty Registry ready for Register calls.
func NewRegistry() *Registry {
	return &Registry{
		generators: make(map[schema.SemanticType]TypeGenerator),
	}
}

// Register adds or overrides a generator for the given semantic type.
// It is safe to call before generation begins (not safe concurrent with Generate).
func (registry *Registry) Register(semantic schema.SemanticType, generator TypeGenerator) {
	registry.generators[semantic] = generator
}

// GeneratorFor returns the generator registered for the given semantic type.
// The second return value is false when no generator is registered for this type.
func (registry *Registry) GeneratorFor(semantic schema.SemanticType) (TypeGenerator, bool) {
	generator, ok := registry.generators[semantic]
	return generator, ok
}

// defaultRegistry is pre-populated with all built-in semantic generators.
var defaultRegistry *Registry

func init() {
	defaultRegistry = NewRegistry()
	defaultRegistry.Register(schema.SemanticFirstName, firstNameGenerator{})
	defaultRegistry.Register(schema.SemanticLastName, lastNameGenerator{})
	defaultRegistry.Register(schema.SemanticFullName, fullNameGenerator{})
	defaultRegistry.Register(schema.SemanticEmail, emailGenerator{})
	defaultRegistry.Register(schema.SemanticPhone, phoneGenerator{})
	defaultRegistry.Register(schema.SemanticAddress, addressGenerator{})
	defaultRegistry.Register(schema.SemanticCity, pickerGenerator{values: cities})
	defaultRegistry.Register(schema.SemanticCountry, pickerGenerator{values: countries})
	defaultRegistry.Register(schema.SemanticState, pickerGenerator{values: states})
	defaultRegistry.Register(schema.SemanticZip, zipGenerator{})
	defaultRegistry.Register(schema.SemanticURL, urlGenerator{})
	defaultRegistry.Register(schema.SemanticDescription, sentenceGenerator{})
	defaultRegistry.Register(schema.SemanticTitle, titleGenerator{})
	defaultRegistry.Register(schema.SemanticStatus, pickerGenerator{values: statuses})
	defaultRegistry.Register(schema.SemanticColor, pickerGenerator{values: colors})
	defaultRegistry.Register(schema.SemanticCategory, pickerGenerator{values: categories})
	defaultRegistry.Register(schema.SemanticCode, codeGenerator{})
	defaultRegistry.Register(schema.SemanticSlug, slugGenerator{})
}

// semanticGeneratorFor delegates to the default registry for backward compatibility.
// New code should use GenerationContext.Registry instead.
func semanticGeneratorFor(semantic schema.SemanticType) (TypeGenerator, bool) {
	return defaultRegistry.GeneratorFor(semantic)
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
