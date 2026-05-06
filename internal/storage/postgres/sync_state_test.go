package postgres

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/anh-pham191/finance-analysis/internal/ports"
)

func TestSyncStateGetErrorTranslatesSQLNoRowsToPortNotFound(t *testing.T) {
	t.Parallel()

	err := syncStateGetError(sql.ErrNoRows)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("error = %v, want ports.ErrNotFound", err)
	}
}
