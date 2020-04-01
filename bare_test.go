package repo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestBare(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	t.Logf("Testing in %s", path)
	defer os.RemoveAll(path)
	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	_, err := CloneBare(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	repo, err := NewBare(path, WithVerbose())
	repo.repo.logger = t.Logf
	require.NoError(t, err)

	files, err := repo.ListFiles()
	require.NoError(t, err)
	assert.True(t, len(files) > 1)
	t.Logf("%+v", files)

	readmeReader, err := repo.ReadFile("README.md")
	require.NoError(t, err)
	readmeContent, err := ioutil.ReadAll(readmeReader)
	require.NoError(t, err)
	t.Logf("%s", string(readmeContent))

}
