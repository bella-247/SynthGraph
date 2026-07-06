package generator

import (
	"math/rand/v2"
	"strings"

	"synthgraph/internal/schema"
)

type sentenceGenerator struct{}

func (sentenceGenerator) Generate(column *schema.Column, _ int, rng *rand.Rand) (any, error) {
	maxLen := column.Length
	if maxLen <= 0 {
		maxLen = 120
	}
	return buildSentence(rng, maxLen, loremWords), nil
}

type titleGenerator struct{}

func (titleGenerator) Generate(column *schema.Column, _ int, rng *rand.Rand) (any, error) {
	maxLen := column.Length
	if maxLen <= 0 {
		maxLen = 80
	}
	prefix := titlePrefixes[rng.IntN(len(titlePrefixes))]
	subject := titleSubjects[rng.IntN(len(titleSubjects))]

	result := prefix + " " + subject
	if len(result) <= maxLen {
		return result, nil
	}

	if len(subject) <= maxLen {
		return subject, nil
	}

	return buildSentence(rng, maxLen, loremWords), nil
}

func buildSentence(rng *rand.Rand, maxLen int, words []string) string {
	wordCount := rng.IntN(10) + 3
	length := 0
	result := make([]string, 0, wordCount)
	for i := 0; i < wordCount; i++ {
		w := words[rng.IntN(len(words))]
		if i > 0 && length+1+len(w) > maxLen {
			break
		}
		if i > 0 {
			length++
		}
		if length+len(w) > maxLen {
			break
		}
		result = append(result, w)
		length += len(w)
	}
	if len(result) == 0 {
		return words[rng.IntN(len(words))]
	}
	return strings.Join(result, " ")
}
