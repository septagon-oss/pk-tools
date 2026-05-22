package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOSSSourceDoesNotEmbedPrivateImportRoots(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob scaffold sources: %v", err)
	}
	forbidden := "github.com/" + "septagon-dev"
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("%s embeds private import root %q", file, forbidden)
		}
	}
}
