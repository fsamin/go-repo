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

	require.NoError(t, b.Repo().FetchBranchWithoutCheckout(context.TODO(), "origin", "feat"))
	assert.Equal(t, want, execGitIn(t, path, "rev-parse", "refs/heads/feat"))

	// Non-fast-forward update (force-push): the "+" refspec must allow it
	execGitIn(t, fixture, "commit", "-q", "--amend", "-m", "feat commit amended")
	wantAmended := execGitIn(t, fixture, "rev-parse", "feat")
	require.NotEqual(t, want, wantAmended)

	require.NoError(t, b.Repo().FetchBranchWithoutCheckout(context.TODO(), "origin", "feat"))
	assert.Equal(t, wantAmended, execGitIn(t, path, "rev-parse", "refs/heads/feat"))
}

func TestDescribeRefOnBareRepo(t *testing.T) {
	fixture := createLocalFixtureRepo(t)
	execGitIn(t, fixture, "tag", "v1.0.0")

	path := filepath.Join(os.TempDir(), "testdata", t.Name(), "clone")
	defer os.RemoveAll(path)
	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	_, err := CloneBare(context.TODO(), path, "file://"+fixture, WithFilter("blob:none"))
	require.NoError(t, err)
	b, err := NewBare(context.TODO(), path)
	require.NoError(t, err)

	// One commit on a feature branch after the clone
	execGitIn(t, fixture, "checkout", "-q", "-b", "feat")
	require.NoError(t, ioutil.WriteFile(filepath.Join(fixture, "feat.go"), []byte("package feat"), os.FileMode(0644)))
	execGitIn(t, fixture, "add", ".")
	execGitIn(t, fixture, "commit", "-q", "-m", "feat commit")
	require.NoError(t, b.Repo().FetchBranchWithoutCheckout(context.TODO(), "origin", "feat"))

	featHash := "g" + execGitIn(t, path, "rev-parse", "--short", "refs/heads/feat")

	// Empty DirtyMark: no --dirty flag, which git rejects on bare repositories
	opt := DescribeOpt{Long: true, Match: []string{"v[0-9]*"}, Ref: "refs/heads/feat"}
	d, err := b.Repo().Describe(context.TODO(), &opt)
	require.NoError(t, err)
	require.NotNil(t, d.Semver)
	assert.Equal(t, "v1.0.0-1-"+featHash, d.Raw)
	assert.Equal(t, "v1.0.0", d.Tag)
	assert.Equal(t, 1, d.Distance)
	assert.Equal(t, featHash, d.Hash)
	assert.Equal(t, featHash, d.Suffix)
	assert.Equal(t, "1.0.0+1."+featHash, d.SemverString)
	assert.False(t, d.Dirty)

	// A tag created after the clone must be seen: Describe fetches tags itself
	execGitIn(t, fixture, "tag", "v1.1.0")
	d, err = b.Repo().Describe(context.TODO(), &opt)
	require.NoError(t, err)
	require.NotNil(t, d.Semver)
	assert.Equal(t, "v1.1.0-0-"+featHash, d.Raw)
	assert.Equal(t, "v1.1.0", d.Tag)
	assert.Equal(t, 0, d.Distance)
	assert.Equal(t, "1.1.0", d.SemverString)

	// Ref selection: master is still described as v1.0.0
	masterHash := "g" + execGitIn(t, path, "rev-parse", "--short", "refs/heads/master")
	d, err = b.Repo().Describe(context.TODO(), &DescribeOpt{Long: true, Match: []string{"v[0-9]*"}, Ref: "refs/heads/master"})
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0-0-"+masterHash, d.Raw)
	assert.Equal(t, "v1.0.0", d.Tag)
	assert.Equal(t, 0, d.Distance)
}

func TestDiffMergeBaseOnBareRepo(t *testing.T) {
	fixture := createLocalFixtureRepo(t)
	path := filepath.Join(os.TempDir(), "testdata", t.Name(), "clone")
	defer os.RemoveAll(path)
	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	_, err := CloneBare(context.TODO(), path, "file://"+fixture, WithFilter("blob:none"))
	require.NoError(t, err)
	b, err := NewBare(context.TODO(), path)
	require.NoError(t, err)

	// feat: one modification, one rename, one addition
	execGitIn(t, fixture, "checkout", "-q", "-b", "feat")
	require.NoError(t, ioutil.WriteFile(filepath.Join(fixture, "README.md"), []byte("# modified"), os.FileMode(0644)))
	execGitIn(t, fixture, "mv", "main.go", "renamed.go")
	require.NoError(t, ioutil.WriteFile(filepath.Join(fixture, "handler.go"), []byte("package handler"), os.FileMode(0644)))
	execGitIn(t, fixture, "add", ".")
	execGitIn(t, fixture, "commit", "-q", "-m", "feat changes")

	// master diverges: its own commits must not appear in the changeset
	execGitIn(t, fixture, "checkout", "-q", "master")
	require.NoError(t, ioutil.WriteFile(filepath.Join(fixture, "master-only.go"), []byte("package master"), os.FileMode(0644)))
	execGitIn(t, fixture, "add", ".")
	execGitIn(t, fixture, "commit", "-q", "-m", "master moves on")

	require.NoError(t, b.Repo().FetchBranchWithoutCheckout(context.TODO(), "origin", "feat"))
	require.NoError(t, b.Repo().FetchBranchWithoutCheckout(context.TODO(), "origin", "master"))

	missingBefore := execGitIn(t, path, "rev-list", "--objects", "--missing=print", "refs/heads/feat")

	files, err := b.Repo().DiffMergeBase(context.TODO(), "refs/heads/master", "refs/heads/feat")
	require.NoError(t, err)

	require.Len(t, files, 4)
	assert.Equal(t, "M", files["README.md"].Status)
	assert.Equal(t, "D", files["main.go"].Status, "a rename must be reported as delete+add")
	assert.Equal(t, "A", files["renamed.go"].Status, "a rename must be reported as delete+add")
	assert.Equal(t, "A", files["handler.go"].Status)
	assert.NotContains(t, files, "master-only.go", "commits on the from side must not appear")

	// --no-renames skips similarity computation: no blob was lazily fetched
	assert.Equal(t, missingBefore, execGitIn(t, path, "rev-list", "--objects", "--missing=print", "refs/heads/feat"))
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

func TestNewBareChecksExactPath(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(root, "repo.git")
	require.NoError(t, os.MkdirAll(bare, os.FileMode(0755)))
	require.NoError(t, exec.Command("git", "init", "-q", "--bare", bare).Run())

	_, err := NewBare(context.TODO(), bare)
	require.NoError(t, err, "a bare repository opens from its own path")

	// An empty directory inside a bare repository is not a repository itself:
	// the parent must not be picked up in its place.
	inside := filepath.Join(bare, "cache", "entry")
	require.NoError(t, os.MkdirAll(inside, os.FileMode(0755)))
	_, err = NewBare(context.TODO(), inside)
	require.Error(t, err)

	_, err = NewBare(context.TODO(), filepath.Join(root, "empty"))
	require.Error(t, err, "a missing path is not a bare repository")
}

func TestRevParse(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	run("commit", "-q", "--allow-empty", "-m", "init")
	run("tag", "-a", "v1", "-m", "annotated")
	run("tag", "light")
	sha := run("rev-parse", "HEAD")

	r, err := New(context.TODO(), root)
	require.NoError(t, err)

	for _, rev := range []string{"refs/tags/v1", "refs/tags/light", "HEAD", sha} {
		got, err := r.RevParse(context.TODO(), rev)
		require.NoError(t, err, rev)
		require.Equal(t, sha, got, "%s must peel to the commit", rev)
	}
	_, err = r.RevParse(context.TODO(), "refs/tags/missing")
	require.Error(t, err)
}
