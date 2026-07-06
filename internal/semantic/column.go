package semantic

import (
	"unicode"

	"synthgraph/internal/schema"
)

type patternEntry struct {
	all      []string
	any      []string
	exclude  []string
	semantic schema.SemanticType
}

var columnPatterns = []patternEntry{
	{all: []string{"first", "name"}, semantic: schema.SemanticFirstName},
	{all: []string{"last", "name"}, semantic: schema.SemanticLastName},
	{all: []string{"full", "name"}, semantic: schema.SemanticFullName},
	{any: []string{"email", "mail"}, exclude: []string{"password"}, semantic: schema.SemanticEmail},
	{any: []string{"phone", "telephone", "tel", "mobile", "cell"}, semantic: schema.SemanticPhone},
	{any: []string{"address", "street", "addr"}, semantic: schema.SemanticAddress},
	{all: []string{"city"}, semantic: schema.SemanticCity},
	{all: []string{"country"}, semantic: schema.SemanticCountry},
	{any: []string{"state", "province"}, semantic: schema.SemanticState},
	{any: []string{"code", "sku"}, exclude: []string{"password", "zip", "postal", "zipcode"}, semantic: schema.SemanticCode},
	{any: []string{"zip", "postal", "zipcode"}, semantic: schema.SemanticZip},
	{any: []string{"url", "website", "homepage", "web"}, semantic: schema.SemanticURL},
	{any: []string{"description", "comment", "note", "details"}, semantic: schema.SemanticDescription},
	{any: []string{"title", "subject", "heading"}, semantic: schema.SemanticTitle},
	{all: []string{"status"}, semantic: schema.SemanticStatus},
	{any: []string{"color", "colour"}, semantic: schema.SemanticColor},
	{any: []string{"category", "class"}, exclude: []string{"password", "mime", "content", "file"}, semantic: schema.SemanticCategory},
	{all: []string{"slug"}, semantic: schema.SemanticSlug},
	{all: []string{"name"}, semantic: schema.SemanticFullName},
}

func tokenize(name string) []string {
	var tokens []string
	var current []rune

	for _, r := range name {
		switch {
		case r == '_' || r == '-' || r == ' ':
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
		case unicode.IsUpper(r) && len(current) > 0 && unicode.IsLower(current[len(current)-1]):
			tokens = append(tokens, string(current))
			current = []rune{unicode.ToLower(r)}
		default:
			current = append(current, unicode.ToLower(r))
		}
	}
	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}
	return tokens
}

func tokensMatchAny(tokens, words []string) bool {
	for _, w := range words {
		for _, t := range tokens {
			if t == w {
				return true
			}
		}
	}
	return false
}

func tokensMatchAll(tokens, words []string) bool {
	for _, w := range words {
		found := false
		for _, t := range tokens {
			if t == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func matchColumnSemantic(name string) schema.SemanticType {
	tokens := tokenize(name)

	for _, entry := range columnPatterns {
		if tokensMatchAny(tokens, entry.exclude) {
			continue
		}
		if len(entry.all) > 0 && !tokensMatchAll(tokens, entry.all) {
			continue
		}
		if len(entry.any) > 0 && !tokensMatchAny(tokens, entry.any) {
			continue
		}
		if len(entry.all) == 0 && len(entry.any) == 0 {
			continue
		}
		return entry.semantic
	}

	return schema.SemanticNone
}

func ResolveColumns(model *schema.Model) {
	for _, table := range model.Tables {
		for i := range table.Columns {
			table.Columns[i].Semantic = matchColumnSemantic(table.Columns[i].Name)
		}
	}
}
