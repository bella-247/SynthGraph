package generator

import (
	"math/rand/v2"

	"synthgraph/internal/schema"
)

// GeoLocation is a correlated city/state/zip triad used for consistent
// geographic data within a single row.
type GeoLocation struct {
	City  string
	State string
	Zip   string
}

// geoLocations provides pre-linked city/state/zip records so that the
// generator can produce consistent geographic data across correlated columns.
var geoLocations = []GeoLocation{
	{"New York", "New York", "10001"},
	{"Los Angeles", "California", "90001"},
	{"Chicago", "Illinois", "60601"},
	{"Houston", "Texas", "77001"},
	{"Phoenix", "Arizona", "85001"},
	{"Philadelphia", "Pennsylvania", "19101"},
	{"San Antonio", "Texas", "78201"},
	{"San Diego", "California", "92101"},
	{"Dallas", "Texas", "75201"},
	{"San Jose", "California", "95101"},
	{"Austin", "Texas", "73301"},
	{"Jacksonville", "Florida", "32099"},
	{"Fort Worth", "Texas", "76101"},
	{"Columbus", "Ohio", "43085"},
	{"Charlotte", "North Carolina", "28201"},
	{"Indianapolis", "Indiana", "46201"},
	{"San Francisco", "California", "94101"},
	{"Seattle", "Washington", "98101"},
	{"Denver", "Colorado", "80201"},
	{"Nashville", "Tennessee", "37201"},
	{"Portland", "Oregon", "97201"},
	{"Miami", "Florida", "33101"},
	{"Atlanta", "Georgia", "30301"},
	{"Boston", "Massachusetts", "02101"},
}

// NamePair is a correlated first/last name pair, reused across first_name,
// last_name, and full_name columns within a single row.
type NamePair struct {
	First string
	Last  string
}

// namePairs provides pre-linked first/last name pairs for consistent naming.
var namePairs = []NamePair{
	{"James", "Smith"}, {"Mary", "Johnson"}, {"John", "Williams"},
	{"Patricia", "Brown"}, {"Robert", "Jones"}, {"Jennifer", "Garcia"},
	{"Michael", "Miller"}, {"Linda", "Davis"}, {"David", "Rodriguez"},
	{"Elizabeth", "Martinez"}, {"William", "Hernandez"}, {"Barbara", "Lopez"},
	{"Richard", "Gonzalez"}, {"Susan", "Wilson"}, {"Joseph", "Anderson"},
	{"Jessica", "Thomas"}, {"Thomas", "Taylor"}, {"Sarah", "Moore"},
	{"Christopher", "Jackson"}, {"Karen", "Martin"}, {"Charles", "Lee"},
	{"Lisa", "Perez"}, {"Daniel", "Thompson"}, {"Nancy", "White"},
}

// correlationGroup describes which correlation groups a table needs.
type correlationGroup struct {
	hasGeo  bool
	hasName bool
}

// detectCorrelationGroups scans the table's columns for known correlation
// groups (geo, name) and returns which groups are present.
func detectCorrelationGroups(table *schema.Table) correlationGroup {
	var groups correlationGroup
	for _, col := range table.Columns {
		switch col.Semantic {
		case schema.SemanticCity, schema.SemanticState, schema.SemanticZip:
			groups.hasGeo = true
		case schema.SemanticFirstName, schema.SemanticLastName, schema.SemanticFullName:
			groups.hasName = true
		}
	}
	return groups
}

// precomputeRowValues returns a map of column name → pre-computed correlated
// value for a single row. Columns not part of any correlation group are omitted
// from the map — they fall through to the normal generator path.
func precomputeRowValues(table *schema.Table, rng *rand.Rand) map[string]any {
	groups := detectCorrelationGroups(table)
	if !groups.hasGeo && !groups.hasName {
		return nil
	}

	values := make(map[string]any, 6)

	if groups.hasGeo {
		loc := geoLocations[rng.IntN(len(geoLocations))]
		for _, col := range table.Columns {
			switch col.Semantic {
			case schema.SemanticCity:
				values[col.Name] = loc.City
			case schema.SemanticState:
				values[col.Name] = loc.State
			case schema.SemanticZip:
				values[col.Name] = loc.Zip
			}
		}
	}

	if groups.hasName {
		pair := namePairs[rng.IntN(len(namePairs))]
		for _, col := range table.Columns {
			switch col.Semantic {
			case schema.SemanticFirstName:
				values[col.Name] = pair.First
			case schema.SemanticLastName:
				values[col.Name] = pair.Last
			case schema.SemanticFullName:
				values[col.Name] = pair.First + " " + pair.Last
			}
		}
	}

	return values
}
