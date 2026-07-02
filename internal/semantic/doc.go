// Package semantic enriches a graph.Graph with inferred meaning.
//
// The semantic layer is the "brain" of SynthGraph — it reads the structural
// graph and infers what every node and relationship *means*, not just what
// it is. It is a purely additive analysis step: no existing graph data is
// modified, and the original graph.Graph is always preserved in full.
//
// # Architecture position
//
//	Parser → schema.Model → graph.Build → graph.Graph
//	                                           │
//	                                           ▼
//	                                     semantic.Build
//	                                           │
//	                                           ▼
//	                                     SemanticGraph
//	                                           │
//	                                    ┌──────┴──────┐
//	                                    ▼             ▼
//	                              Analysis        AI docs
//
// # How inferences work
//
// Each semantic property is produced by one or more Rules. A Rule is a
// self-contained piece of inference logic that examines a node in the context
// of the full graph and returns a slice of Inferences. Each Inference carries:
//
//   - What was concluded (Kind)
//   - How confident the engine is (Confidence: 0.0–1.0)
//   - Why it concluded it (Evidence: human-readable reasons)
//
// This design makes every conclusion explainable — future AI layers can relay
// the evidence directly to users ("I inferred this is a junction table because
// all primary key columns are foreign keys").
//
// # Extensibility
//
// New inference capabilities are added by implementing the Rule interface.
// No existing files need modification. Domain-specific rule sets (healthcare,
// finance, DDD patterns) can be compiled in separately, making SynthGraph a
// semantic analysis platform rather than a fixed inference engine.
package semantic
