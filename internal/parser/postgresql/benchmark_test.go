package postgresql

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkParse(b *testing.B) {
	fixtures, err := filepath.Glob("../../../testdata/schemas/*.sql")
	if err != nil || len(fixtures) == 0 {
		b.Skip("no test SQL files found")
	}

	for _, fixturePath := range fixtures {
		sqlBytes, err := os.ReadFile(fixturePath)
		if err != nil {
			b.Fatalf("reading %s: %v", fixturePath, err)
		}
		parser := New()
		name := filepath.Base(fixturePath)

		b.Run(name, func(b *testing.B) {
			for range b.N {
				_, err := parser.Parse(sqlBytes)
				if err != nil {
					b.Fatalf("Parse: %v", err)
				}
			}
		})
	}
}
