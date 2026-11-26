package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	require.NoError(t, WriteFileAtomic(path, []byte("one")))
	require.NoError(t, WriteFileAtomic(path, []byte("two")))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "two", string(data))
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1)
}

func TestWriteFileAtomic_MissingDirectory(t *testing.T) {
	assert.Error(t, WriteFileAtomic(filepath.Join(t.TempDir(), "missing", "data.json"), []byte("x")))
}
