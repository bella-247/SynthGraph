package server

import "synthgraph/internal/schema"

type parseRequest struct {
	SQL string `json:"sql"`
}

type parseResponse struct {
	Tables   int            `json:"tables"`
	Enums    int            `json:"enums"`
	Model    *schema.Model  `json:"model"`
	Warnings []string       `json:"warnings,omitempty"`
}

type graphResponse struct {
	Nodes []nodeJSON `json:"nodes"`
	Edges []edgeJSON `json:"edges"`
}

type nodeJSON struct {
	ID         string `json:"id"`
	Table      string `json:"table"`
	Columns    int    `json:"columns"`
	IsJunction bool   `json:"is_junction"`
}

type edgeJSON struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Label    string `json:"label"`
	Nullable bool   `json:"nullable"`
}

type semanticResponse struct {
	Nodes []semNodeJSON `json:"nodes"`
	Edges []semEdgeJSON `json:"edges"`
}

type semNodeJSON struct {
	ID    string   `json:"id"`
	Roles []string `json:"roles"`
}

type semEdgeJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type generationRequest struct {
	Input      string `json:"input"`
	Rows       int    `json:"rows"`
	Seed       int64  `json:"seed"`
	Format     string `json:"format"`
	SchemaName string `json:"schema_name"`
}
