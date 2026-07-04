package parser

// Registry manages available SchemaParser implementations.
type Registry struct {
	parsers map[string]SchemaParser
}

// NewRegistry creates an empty registry.
// Call Register to add parsers before use.
func NewRegistry() *Registry {
	return &Registry{
		parsers: make(map[string]SchemaParser),
	}
}

// Register adds a parser to the registry by name.
func (r *Registry) Register(p SchemaParser) {
	r.parsers[p.Name()] = p
}

// Names returns the names of all registered parsers.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.parsers))
	for name := range r.parsers {
		names = append(names, name)
	}
	return names
}

// Get returns a parser by name, or an error if not found.
func (r *Registry) Get(name string) (SchemaParser, error) {
	p, ok := r.parsers[name]
	if !ok {
		return nil, &ParserError{Name: name}
	}
	return p, nil
}

// ParserError indicates a parser was not found in the registry.
type ParserError struct {
	Name string
}

func (e *ParserError) Error() string {
	return "no parser registered: " + e.Name
}
