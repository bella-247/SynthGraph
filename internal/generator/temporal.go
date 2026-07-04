package generator

import (
	"math/rand/v2"
	"time"

	"synthgraph/internal/schema"
)

// timestampGenerator generates random timestamp strings.
type timestampGenerator struct{}

func (generator timestampGenerator) Generate(column *schema.Column, rowIndex int, rng *rand.Rand) (any, error) {
	minTime := int64(1577836800) // 2020-01-01
	maxTime := int64(1798761600) // 2026-12-31
	rangeSize := maxTime - minTime + 1
	unixTime := minTime + rng.Int64N(rangeSize)
	return time.Unix(unixTime, 0).UTC().Format("2006-01-02 15:04:05"), nil
}
