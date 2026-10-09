package evidence

import (
	"fmt"
	"os"
	"testing"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/internal/testenv"
)

// Initialize the explicit test database while holding the shared suite lock.
func TestMain(m *testing.M) {
	os.Exit(testenv.RunPG(func() int { return runEvidenceSuite(m) }))
}

func runEvidenceSuite(m *testing.M) int {
	if os.Getenv("ARTEX_PG_DSN") == "" {
		return m.Run()
	}
	pg, err := db.Open(os.Getenv("ARTEX_PG_DSN"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "evidence: cannot initialize explicit test database (error type %T)\n", err)
		return 1
	}
	defer pg.Close()
	return m.Run()
}
