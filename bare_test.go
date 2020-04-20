package repo

import (
	"context"
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
	_, err := CloneBare(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	repo, err := NewBare(context.TODO(), path, WithVerbose(t.Logf))
	repo.repo.logger = t.Logf
	require.NoError(t, err)

	files, err := repo.ListFiles(context.TODO())
	require.NoError(t, err)
	assert.True(t, len(files) > 1)
	t.Logf("%+v", files)

	size, err := repo.FileSize(context.TODO(), "README.md")
	require.NoError(t, err)
	assert.NotEqual(t, -1, size)
	assert.True(t, size > 100)

	readmeReader, err := repo.ReadFile(context.TODO(), "README.md")
	require.NoError(t, err)
	readmeContent, err := ioutil.ReadAll(readmeReader)
	require.NoError(t, err)
	t.Logf("%s", string(readmeContent))

}
