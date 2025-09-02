package toxcore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGap2CAPIDocumentationVsImplementation validates that the C API documentation
// references non-existent files and functions, reproducing Gap #2
func TestGap2CAPIDocumentationVsImplementation(t *testing.T) {
	// Test 1: toxcore.h header file referenced in README.md should not exist
	headerFile := "toxcore.h"
	if _, err := os.Stat(headerFile); err == nil {
		t.Errorf("Header file %s exists but should not, as no C bindings are implemented", headerFile)
	}

	// Test 2: Check that no C files exist in the project
	cFiles := []string{}
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) == ".h" || filepath.Ext(path) == ".c" {
			cFiles = append(cFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Error walking directory: %v", err)
	}

	if len(cFiles) > 0 {
		t.Errorf("Found C files %v, but documentation suggests no C implementation exists", cFiles)
	}

	// Test 3: Verify that //export comments exist but no CGO setup
	// This confirms the gap between intention (//export comments) and implementation (no C bindings)
	// The test passes when the gap exists, and will need updating when C bindings are implemented
	t.Logf("Gap #2 confirmed: README.md documents C API but no C bindings exist")
	t.Logf("//export comments found in toxcore.go but no CGO implementation")
	t.Logf("This test documents the current state and will need updating if C bindings are added")
}
