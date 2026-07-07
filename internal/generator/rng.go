package generator

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"strconv"

	"synthgraph/internal/schema"
)

// newTableRNG creates a deterministic RNG for a specific table.
// Seed = FNV-64a("globalSeed:tableName").
func newTableRNG(globalSeed uint64, tableName string) *rand.Rand {
	hash := fnv.New64a()
	hash.Write([]byte(strconv.FormatUint(globalSeed, 10)))
	hash.Write([]byte(":"))
	hash.Write([]byte(tableName))
	seed := hash.Sum64()

	high := seed ^ 0x9e3779b97f4a7c15
	low := seed ^ 0xbf58476d1ce4e5b9
	return rand.New(rand.NewPCG(high, low))
}

// generateUUID produces a random UUID v4 string using the given RNG.
func generateUUID(rng *rand.Rand) string {
	var buffer [16]byte
	binary.LittleEndian.PutUint64(buffer[0:8], rng.Uint64())
	binary.LittleEndian.PutUint64(buffer[8:16], rng.Uint64())

	// Set version 4 (random) — RFC 4122, section 4.4.
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	// Set variant (10xx) — RFC 4122.
	buffer[8] = (buffer[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buffer[0:4],
		buffer[4:6],
		buffer[6:8],
		buffer[8:10],
		buffer[10:16],
	)
}

// randomString generates a random alphanumeric string of the given length.
func randomString(length int, rng *rand.Rand) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[rng.IntN(len(chars))]
	}
	return string(result)
}

// buildEnumValues builds a map from enum type name to its list of values.
func buildEnumValues(model *schema.Model) map[string][]string {
	values := make(map[string][]string, len(model.Enums))
	for _, enumType := range model.Enums {
		vals := make([]string, len(enumType.Values))
		copy(vals, enumType.Values)
		values[enumType.Name] = vals
	}
	return values
}
