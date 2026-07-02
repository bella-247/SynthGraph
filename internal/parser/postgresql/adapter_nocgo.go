//go:build !cgo

package postgresql

import (
	"errors"
)

func init() {
	parseSQL = noCgoParse
}

func noCgoParse(text string) ([]Stmt, error) {
	return nil, errors.New("PostgreSQL parser requires CGO. Install gcc/mingw and rebuild with CGO_ENABLED=1")

	// To install CGO support:
	//   Windows: install MinGW-w64 from https://www.mingw-w64.org/
	//   macOS:   xcode-select --install
	//   Linux:   apt install gcc libpq-dev
	//
	// Then:
	//   go get github.com/pganalyze/pg_query_go/v5
	//   CGO_ENABLED=1 go build ./...
}
