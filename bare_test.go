package repo

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneBareWithFilter(t *testing.T) {
	fixture := createLocalFixtureRepo(t)
	path := filepath.Join(os.TempDir(), "testdata", t.Name(), "clone")
	defer os.RemoveAll(path)
	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	_, err := CloneBare(context.TODO(), path, "file://"+fixture, WithFilter("blob:none"))
	require.NoError(t, err)

	_, err = NewBare(context.TODO(), path)
	require.NoError(t, err)

	out, err := exec.Command("git", "-C", path, "config", "--get", "remote.origin.partialclonefilter").Output()
	require.NoError(t, err)
	assert.Equal(t, "blob:none", strings.TrimSpace(string(out)))

	// No worktree and no checkout: every blob must have been filtered out
	out, err = exec.Command("git", "-C", path, "rev-list", "--objects", "--missing=print", "HEAD").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "?", "expected missing (filtered) blobs in bare partial clone")
}

func TestFetchBranchWithoutCheckoutOnBareRepo(t *testing.T) {
	fixture := createLocalFixtureRepo(t)
	path := filepath.Join(os.TempDir(), "testdata", t.Name(), "clone")
	defer os.RemoveAll(path)
	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	_, err := CloneBare(context.TODO(), path, "file://"+fixture, WithFilter("blob:none"))
	require.NoError(t, err)
	b, err := NewBare(context.TODO(), path)
	require.NoError(t, err)

	// The branch is created after the clone: without an explicit refspec a
	// bare repository would never see it
	execGitIn(t, fixture, "checkout", "-q", "-b", "feat")
	require.NoError(t, ioutil.WriteFile(filepath.Join(fixture, "feat.go"), []byte("package feat"), os.FileMode(0644)))
	execGitIn(t, fixture, "add", ".")
	execGitIn(t, fixture, "commit", "-q", "-m", "feat commit")
	want := execGitIn(t, fixture, "rev-parse", "feat")

	require.NoError(t, b.repo.FetchBranchWithoutCheckout(context.TODO(), "origin", "feat"))
	assert.Equal(t, want, execGitIn(t, path, "rev-parse", "refs/heads/feat"))

	// Non-fast-forward update (force-push): the "+" refspec must allow it
	execGitIn(t, fixture, "commit", "-q", "--amend", "-m", "feat commit amended")
	wantAmended := execGitIn(t, fixture, "rev-parse", "feat")
	require.NotEqual(t, want, wantAmended)

	require.NoError(t, b.repo.FetchBranchWithoutCheckout(context.TODO(), "origin", "feat"))
	assert.Equal(t, wantAmended, execGitIn(t, path, "rev-parse", "refs/heads/feat"))
}

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
