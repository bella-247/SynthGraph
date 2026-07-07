package generator

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"

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

	colLower := strings.ToLower(column.Name)
	switch {
	case strings.Contains(colLower, "first") && strings.Contains(colLower, "name"):
		return randomFirstName(rng), nil
	case strings.Contains(colLower, "last") && strings.Contains(colLower, "name"):
		return randomLastName(rng), nil
	case colLower == "name" || colLower == "full_name":
		return randomFirstName(rng) + " " + randomLastName(rng), nil
	case strings.Contains(colLower, "email"):
		name := strings.ToLower(randomFirstName(rng))
		domain := emailDomains[rng.IntN(len(emailDomains))]
		return fmt.Sprintf("%s%d@%s", name, rng.IntN(999)+1, domain), nil
	case strings.Contains(colLower, "phone") || strings.Contains(colLower, "tel"):
		return randomPhone(rng), nil
	case strings.Contains(colLower, "address") || strings.Contains(colLower, "street"):
		return randomAddress(rng), nil
	case strings.Contains(colLower, "city"):
		return cities[rng.IntN(len(cities))], nil
	case strings.Contains(colLower, "country"):
		return countries[rng.IntN(len(countries))], nil
	case strings.Contains(colLower, "state") || strings.Contains(colLower, "province"):
		return states[rng.IntN(len(states))], nil
	case strings.Contains(colLower, "zip") || strings.Contains(colLower, "postal"):
		return fmt.Sprintf("%05d", rng.IntN(90000)+10000), nil
	case strings.Contains(colLower, "url") || strings.Contains(colLower, "website"):
		words := []string{"example", "demo", "test", "sample", "app"}
		return "https://www." + words[rng.IntN(len(words))] + ".com/" + randomString(8, rng), nil
	case strings.Contains(colLower, "description") || strings.Contains(colLower, "comment"):
		return randomSentence(rng, maxLen), nil
	case strings.Contains(colLower, "title") || strings.Contains(colLower, "subject"):
		return randomTitle(rng, maxLen), nil
	case strings.Contains(colLower, "status"):
		statuses := []string{"active", "inactive", "pending", "archived"}
		return statuses[rng.IntN(len(statuses))], nil
	case strings.Contains(colLower, "color"):
		colors := []string{"red", "blue", "green", "black", "white", "yellow"}
		return colors[rng.IntN(len(colors))], nil
	case strings.Contains(colLower, "category") || strings.Contains(colLower, "type"):
		categories := []string{"A", "B", "C", "Premium", "Standard", "Basic"}
		return categories[rng.IntN(len(categories))], nil
	case strings.Contains(colLower, "code") || strings.Contains(colLower, "sku"):
		return randomCode(rng), nil
	case strings.Contains(colLower, "slug"):
		return randomSlug(rng), nil
	default:
		length := rng.IntN(maxLen) + 1
		if length < 1 {
			length = 1
		}
		return randomString(length, rng), nil
	}
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

// ── Realistic data helpers ────────────────────────────────────────────────

func randomFirstName(rng *rand.Rand) string {
	return firstNames[rng.IntN(len(firstNames))]
}

func randomLastName(rng *rand.Rand) string {
	return lastNames[rng.IntN(len(lastNames))]
}

func randomPhone(rng *rand.Rand) string {
	area := rng.IntN(800) + 200
	prefix := rng.IntN(1000)
	line := rng.IntN(10000)
	return fmt.Sprintf("(%d) %03d-%04d", area, prefix, line)
}

func randomAddress(rng *rand.Rand) string {
	number := rng.IntN(9999) + 1
	streets := []string{"Main St", "Oak Ave", "Elm St", "Park Blvd", "Broadway", "Lake Dr", "Cedar Ln", "Maple Rd"}
	return fmt.Sprintf("%d %s", number, streets[rng.IntN(len(streets))])
}

func randomSentence(rng *rand.Rand, maxLen int) string {
	words := []string{"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing", "elit", "sed", "do", "eiusmod", "tempor", "incididunt", "ut", "labore", "et", "dolore", "magna", "aliqua", "enim", "ad", "minim", "veniam", "quis", "nostrud", "exercitation", "ullamco", "laboris", "nisi", "aliquip"}
	var result []string
	wordCount := rng.IntN(10) + 3
	for i := 0; i < wordCount; i++ {
		result = append(result, words[rng.IntN(len(words))])
	}
	sentence := strings.Join(result, " ")
	if len(sentence) > maxLen {
		sentence = sentence[:maxLen]
	}
	return sentence
}

func randomTitle(rng *rand.Rand, maxLen int) string {
	prefixes := []string{"Analysis of", "Report on", "Study of", "Update for", "Summary of"}
	subjects := []string{"Q4 Performance", "User Growth", "Revenue", "Traffic", "Conversion", "Retention", "Satisfaction"}
	title := prefixes[rng.IntN(len(prefixes))] + " " + subjects[rng.IntN(len(subjects))]
	if len(title) > maxLen {
		title = title[:maxLen]
	}
	return title
}

func randomCode(rng *rand.Rand) string {
	letters := randomString(3, rng)
	digits := rng.IntN(10000)
	return strings.ToUpper(letters) + fmt.Sprintf("%04d", digits)
}

func randomSlug(rng *rand.Rand) string {
	words := []string{"hello", "world", "foo", "bar", "demo", "test", "sample", "app", "user", "admin"}
	count := rng.IntN(3) + 1
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		parts[i] = words[rng.IntN(len(words))]
	}
	return strings.Join(parts, "-")
}

