package repo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- validateRef tests ---

func TestValidateRef_Valid(t *testing.T) {
	cases := []string{
		"main",
		"feature/my-branch",
		"v1.0.0",
		"abc123",
		"refs/heads/main",
		"origin",
		"HEAD",
		"my-tag",
		"release/2.0",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			got, err := validateRef(tc)
			require.NoError(t, err)
			assert.Equal(t, tc, got)
		})
	}
}

func TestValidateRef_Trimmed(t *testing.T) {
	got, err := validateRef("  main  ")
	require.NoError(t, err)
	assert.Equal(t, "main", got)
}

func TestValidateRef_Empty(t *testing.T) {
	_, err := validateRef("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty git reference")

	_, err = validateRef("   ")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty git reference")
}

func TestValidateRef_OptionInjection(t *testing.T) {
	cases := []string{
		"--upload-pack=evil",
		"-c",
		"--exec=sh",
		"-o",
		"--config=core.sshCommand=evil",
		"--recurse-submodules=evil",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			_, err := validateRef(tc)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "must not start with '-'")
		})
	}
}

// --- validatePath tests ---

func TestValidatePath_Valid(t *testing.T) {
	base := t.TempDir()

	cases := []string{
		"file.txt",
		"subdir/file.txt",
		"a/b/c/d.go",
		"hooks/pre-commit",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			got, err := validatePath(base, tc)
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(got, base))
		})
	}
}

func TestValidatePath_Traversal(t *testing.T) {
	base := t.TempDir()

	cases := []string{
		"../etc/passwd",
		"../../etc/shadow",
		"subdir/../../etc/passwd",
		"../../../../../../../etc/passwd",
		"hooks/../../../etc/passwd",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			_, err := validatePath(base, tc)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "escapes base directory")
		})
	}
}

func TestValidatePath_StaysInBase(t *testing.T) {
	base := t.TempDir()

	// This path uses .. but stays within base
	subdir := filepath.Join(base, "a", "b")
	require.NoError(t, os.MkdirAll(subdir, 0755))

	got, err := validatePath(base, "a/b/../c.txt")
	require.NoError(t, err)
	expected := filepath.Join(base, "a", "c.txt")
	assert.Equal(t, expected, got)
}

// --- sanitizeURL tests ---

func TestSanitizeURL_NoCredentials(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"https plain", "https://github.com/user/repo.git"},
		{"http plain", "http://github.com/user/repo.git"},
		{"ssh format", "git@github.com:user/repo.git"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeURL(tc.input)
			assert.Equal(t, tc.input, got)
		})
	}
}

func TestSanitizeURL_WithCredentials(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		contains string // must NOT contain
	}{
		{
			"https with user:password",
			"https://user:s3cret@github.com/user/repo.git",
			"s3cret",
		},
		{
			"https with user only",
			"https://user@github.com/user/repo.git",
			"user@github.com",
		},
		{
			"ssh with user:password",
			"ssh://user:s3cret@example.com/repo.git",
			"s3cret",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeURL(tc.input)
			assert.NotContains(t, got, tc.contains)
		})
	}
}

// --- sanitizeErrorMessage tests ---

func TestSanitizeErrorMessage_NoURLs(t *testing.T) {
	msg := "fatal: repository not found"
	assert.Equal(t, msg, sanitizeErrorMessage(msg))
}

func TestSanitizeErrorMessage_WithCredentials(t *testing.T) {
	msg := "fatal: unable to access 'https://user:password@github.com/repo.git': The requested URL returned error: 403"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "password")
	assert.Contains(t, result, "github.com")
	assert.Contains(t, result, "fatal")
}

func TestSanitizeErrorMessage_MultipleURLs(t *testing.T) {
	msg := "error pushing https://user:pass1@github.com/a.git and https://admin:pass2@github.com/b.git"
	result := sanitizeErrorMessage(msg)
	assert.NotContains(t, result, "pass1")
	assert.NotContains(t, result, "pass2")
}

// --- shellEscape tests ---

func TestShellEscape(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "/path/to/key", "'/path/to/key'"},
		{"with spaces", "/path/to my/key", "'/path/to my/key'"},
		{"with single quote", "it's a key", "'it'\\''s a key'"},
		{"with double quotes", `path/"key"`, `'path/"key"'`},
		{"with semicolons", "path;rm -rf /", "'path;rm -rf /'"},
		{"with backticks", "path`cmd`", "'path`cmd`'"},
		{"with dollar", "path$HOME/key", "'path$HOME/key'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shellEscape(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// --- Close() cleanup tests ---

func TestClose_NoSSHKey(t *testing.T) {
	r := Repo{path: "/tmp/test"}
	// Close on a repo with no SSH key should be a no-op
	require.NoError(t, r.Close())
}

func TestClose_CleansUpSSHFiles(t *testing.T) {
	tmpDir := t.TempDir()
	keyDir := filepath.Join(tmpDir, "ssh-test")
	require.NoError(t, os.MkdirAll(keyDir, 0700))

	keyFile := filepath.Join(keyDir, "id_rsa")
	require.NoError(t, os.WriteFile(keyFile, []byte("fake-key"), 0600))

	wrapperFile := filepath.Join(keyDir, "gitwrapper")
	require.NoError(t, os.WriteFile(wrapperFile, []byte("#!/bin/sh\n"), 0700))

	r := Repo{
		path: "/tmp/test",
		sshKey: &sshKey{
			filename: keyFile,
			content:  []byte("fake-key"),
		},
	}

	// Files should exist before Close
	_, err := os.Stat(keyFile)
	require.NoError(t, err)
	_, err = os.Stat(wrapperFile)
	require.NoError(t, err)

	require.NoError(t, r.Close())

	// Files should be cleaned up after Close
	_, err = os.Stat(keyFile)
	assert.True(t, os.IsNotExist(err), "SSH key file should be removed")
	_, err = os.Stat(wrapperFile)
	assert.True(t, os.IsNotExist(err), "SSH wrapper script should be removed")

	// Directory should also be removed (since it's now empty)
	_, err = os.Stat(keyDir)
	assert.True(t, os.IsNotExist(err), "SSH key directory should be removed")
}

// --- Git option injection in public methods ---

func TestOptionInjection_Checkout(t *testing.T) {
	// Setup a real git repo so the error comes from validation, not from missing repo
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))
	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	err = r.Checkout(context.TODO(), "--upload-pack=evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestOptionInjection_CheckoutNewBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))
	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	err = r.CheckoutNewBranch(context.TODO(), "--upload-pack=evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestOptionInjection_DeleteBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))
	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	err = r.DeleteBranch(context.TODO(), "--exec=evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestOptionInjection_Push(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))
	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	err = r.Push(context.TODO(), "--receive-pack=evil", "main")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")

	err = r.Push(context.TODO(), "origin", "--receive-pack=evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestOptionInjection_Pull(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))
	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	err = r.Pull(context.TODO(), "--upload-pack=evil", "main")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestOptionInjection_ResetHard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))
	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	err = r.ResetHard(context.TODO(), "--exec=evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

func TestOptionInjection_FetchRemoteBranch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))
	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	err = r.FetchRemoteBranch(context.TODO(), "--upload-pack=evil", "main")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")

	err = r.FetchRemoteBranch(context.TODO(), "origin", "--upload-pack=evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not start with '-'")
}

// --- Path traversal in Open and Write ---

func TestPathTraversal_Open(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))
	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	_, err = r.Open("../../../etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes base directory")
}

func TestPathTraversal_Write(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))
	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	err = r.Write("../../../tmp/evil.txt", strings.NewReader("malicious content"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes base directory")
}

// --- Credential filtering in error messages ---

func TestCredentialFiltering_PushError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))

	r, err := Clone(context.TODO(), path, "https://github.com/fsamin/go-repo.git", WithHTTPAuth("user", "supersecret"))
	require.NoError(t, err)

	require.NoError(t, r.CheckoutNewBranch(context.TODO(), "test-cred-filter"))
	require.NoError(t, r.Write("test.txt", strings.NewReader("test")))
	require.NoError(t, r.Add(context.TODO(), "test.txt"))
	require.NoError(t, r.Commit(context.TODO(), "test"))

	err = r.Push(context.TODO(), "origin", "test-cred-filter")
	// Push will fail (no actual credentials), but error should not contain the password
	if err != nil {
		assert.NotContains(t, err.Error(), "supersecret", "password should be filtered from error messages")
	}
}

func TestCredentialFiltering_CloneError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, os.MkdirAll(path, 0755))

	_, err := Clone(context.TODO(), path, "https://github.com/fsamin/nonexistent-repo.git", WithHTTPAuth("user", "topsecret"))
	// Clone will fail, but error should not contain the password
	if err != nil {
		assert.NotContains(t, err.Error(), "topsecret", "password should be filtered from error messages")
	}
}
