package server

import (
	"os"
	"testing"

	"github.com/Autumn-27/artex/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.RunPG(m.Run))
}
