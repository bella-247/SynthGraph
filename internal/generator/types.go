package generator

import "synthgraph/internal/schema"

// typeGeneratorFor returns the appropriate TypeGenerator for a column type.
// For built-in types, it returns the corresponding standard generator.
// For enum types (not in the built-in set), it returns an enum generator.
func typeGeneratorFor(columnType string, model *schema.Model, enumValues map[string][]string) TypeGenerator {
	if generator, isBuiltIn := builtInGenerators[columnType]; isBuiltIn {
		return generator
	}
	if values, isEnum := enumValues[columnType]; isEnum {
		return enumGenerator(values)
	}
	return unknownTypeGenerator(columnType)
}

// builtInGenerators maps canonical type names to their generators.
var builtInGenerators = map[string]TypeGenerator{
	"int":      intGenerator{min: 1, max: 2147483647},
	"int4":     intGenerator{min: 1, max: 2147483647},
	"int8":     intGenerator{min: 1, max: 9223372036854775807},
	"bigint":   intGenerator{min: 1, max: 9223372036854775807},
	"smallint": intGenerator{min: 1, max: 32767},
	"serial":   intGenerator{min: 1, max: 2147483647},
	"bigserial":   intGenerator{min: 1, max: 9223372036854775807},
	"smallserial": intGenerator{min: 1, max: 32767},

	"varchar":  stringGenerator{},
	"text":     stringGenerator{},
	"char":     stringGenerator{},

	"uuid": uuidGenerator{},

	"timestamp":   timestampGenerator{},
	"timestamptz": timestampGenerator{},
	"date":        timestampGenerator{},
	"time":        timestampGenerator{},
	"timetz":      timestampGenerator{},

	"boolean": boolGenerator{},

	"decimal": decimalGenerator{},
	"numeric": decimalGenerator{},
	"float4":  floatGenerator{},
	"float8":  floatGenerator{},
	"real":    floatGenerator{},
	"double precision": floatGenerator{},

	"json":  jsonGenerator{},
	"jsonb": jsonGenerator{},

	"inet":    inetGenerator{},
	"macaddr": macaddrGenerator{},
}
