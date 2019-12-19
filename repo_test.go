package repo

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)
	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	r, err := New("testdata")
	require.NoError(t, err)
	t.Log(r.path)
}

func TestClone(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)
	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	_, err := Clone(path, "https://github.com/fsamin/go-repo.git", WithHTTPAuth("fsamin", os.Getenv("TEST_TOKEN")))
	require.NoError(t, err)
}

func TestCloneWithError(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	_, err := Clone(path, "https://github.com/fsamin/this-repo-does-not-exist.git")
	t.Logf("Error is : '%v'", err)
	assert.Error(t, err)
}

// This is test rsa key, don't use it please
var testRSAKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAsP/EsqVsJkb+pStaZkm33MYF+t6y1OzM5gP+wO6ckdWCcp9V
OPClEefdlVOZNIX2kmD0+2ySPRcP0HaIBNQ+BFMM3BPLxdv8d+KCokY8D3cb75/A
eOjN9YZuhGIYrkfJT2j121CFQ13YUvQ0G179HqnH91XG54UJRtDcuk5s671ZM9Rn
6diBjnm+c53ueXbRfCi+4VxBTEalKk1MyFk3fZZxgBwoheryOxRoyorig6qCtpTR
Rm6swqNbnpeILVscCLiOmIVD7+tdAZWxZ1K+suiZW/4bO/1w3JNLbmv1PLWO/QmB
SBCkPOOk+bDivkPWQ0SLYerOqrXP5+dg4mxNMwIDAQABAoIBAAbJd/BluXTqSf9p
XykG8J7tlPMesPrLLbwwMQeS3rwU1NCyXWE2kQ3Tt0JvlzNVY7QPNbWiXyUqijez
I9oTjWE7EgYqWCj5G4A5VksEqG7rYU3Z8VZxjtw4UGqRHGMqa4S5AJxtRP7lTVM4
+/qEtO4FEp7gUiU0i7uEbMJUGYcchjamdZ2aREgXHeQpDfqIyRmSofOZyYMd4qMJ
Rn4IRd/K156+93r2L7am0GyyEd0jRVa/96DtYod00E3TYkIp1Ux4XM8qlfVyhwca
0DYffYvgqZVoXDnTkVPSYpRtAbNjrtPPs+BmwW41+2KchqnlW8JzycywrfV1diIv
xcklUkkCgYEA6cDlg542zL776So3VuEG0zP/o1g9sZrTDEmu9uGlW7ZzqfEhzOYi
7NZ/5vai6OCB+rqoi+zXloyxNKAV/t5ZLaH8IzZZh1xquxsmqrob7oEwDNSbZINZ
aJMXbP9+zUsSjJzUIpCScSd+OlVKWRdzuzrNvcioBzdwbygzylHG0K8CgYEAwdgf
qRo8j5dsHA7ux1AIH3SAW2Jvb07nCpKYRXf+Vc/S3wfWlBFMfErA0B0zz79c2A1P
wnUYAg58uGZX9veX8Ghmg601gOB3hH9BWyIxClMc8jAwu5eNzxkf8OQejNtj099j
9NzqtWELM144kPF7SGJ2Ko64K170vPovQvAWhL0CgYEAm4KJLpcDPhOQ4/4B8vqh
38CoQbNi19V4sqQSkoxrxigLqvOQ2RACDC5nyPAsUWGLF5M2rmBSzQWsnqYh+/1Q
ttsdMw/lX/hLyU622r4V9wZbQS3wc14vDTNOUmVnpoxbOtDbEGO+CSmNAKHdZIgF
pnnohmoH30Uyt8C3M9JTwmECgYEAqSocDy4nVcR2g1IAzY2pWRIJhja0OvYnqNFf
85gRLAAO7bZga51hG0L9W2Fwuscslhuf1HrtdbYA38fo0k0mmpXxiM5a19qMUuPf
PFHtbC42H6EwljVfezFY75eUlaZMSzUzfRhh9+H1rWF3if5DcVsD9oXQcYEPoe/P
2OG/NR0CgYB2VSwY326FfYrL9x7c2rKCaV33t4TGRMqvR3zrbdMXBbNghnyqg98E
3aQbRwnUPr5HJLan7aBnEWVb6qELyUyWMhfwyQewlnnMEwa1irHSYArJ/e/bMf3a
tUQx5iDRVkQ61qdoFdD7MR7oQlRkrGRrH1nHDs2dXlpPQ14lGowkGg==
-----END RSA PRIVATE KEY-----`)

func TestCloneFromSSHShouldFailed(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	_, err := Clone(path, "git@github.com:fsamin/go-repo.git", WithSSHAuth(testRSAKey), WithVerbose())
	assert.Error(t, err)
}

func TestCloneFromSSHShouldSuccess(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	pkey, err := ioutil.ReadFile("travis_id_rsa")
	if err != nil {
		t.SkipNow()
	}
	_, err = Clone(path, "git@github.com:fsamin/go-repo.git", WithSSHAuth(pkey), WithVerbose())
	require.NoError(t, err)
}

func TestCloneFromHTTPShouldSuccess(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	_, err := Clone(path, "https://github.com/fsamin/go-repo.git", WithHTTPAuth("fsamin", os.Getenv("TEST_TOKEN")), WithVerbose())
	require.NoError(t, err)
}

func TestCurrentBranch(t *testing.T) {
	r, err := New(".")
	require.NoError(t, err)
	b, err := r.CurrentBranch()
	require.NoError(t, err)
	assert.NotEmpty(t, b)
}

func TestFetchRemoteTag(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	r, err := Clone(path, "https://github.com/fsamin/go-repo.git", WithHTTPAuth("fsamin", os.Getenv("TEST_TOKEN")))
	require.NoError(t, err)

	err = r.FetchRemoteTag("origin", "v0.1.0")
	require.NoError(t, err)

	sha1, err := r.VerifyTag("v0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "7abae771a5d8690b24993971238bbf3b93b6961b", sha1)

	err = r.FetchRemoteTag("origin", "v0.1.1")
	require.NoError(t, err)

	sha1, err = r.VerifyTag("v0.1.1")
	require.NoError(t, err)
	assert.Equal(t, "29d80ac945280d30bb3f2442b1d0e894d8dcb4a1", sha1)
}

func TestFetchRemoteBranch(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)
	err = r.FetchRemoteBranch("origin", "tests")
	require.NoError(t, err)
	b, err := r.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "tests", b)
	err = r.FetchRemoteBranch("origin", "master")
	require.NoError(t, err)
	b, err = r.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "master", b)

}

func TestPull(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)
	err = r.FetchRemoteBranch("origin", "tests")
	require.NoError(t, err)
	b, err := r.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "tests", b)
	err = r.Pull("origin", "tests")
	require.NoError(t, err)
}

func TestResetHard(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)
	err = r.FetchRemoteBranch("origin", "tests")
	require.NoError(t, err)
	err = r.ResetHard("7fc6e6ff62133460b7f288043db6e47edf5dd6aa")
	require.NoError(t, err)
}

func TestNewWithError(t *testing.T) {
	_, err := New(os.TempDir())
	assert.NotNil(t, err)
}

func TestFetchURL(t *testing.T) {
	r, err := New(".")
	require.NoError(t, err)

	u, err := r.FetchURL()
	require.NoError(t, err)

	t.Logf("url: %v", u)

	n, err := r.Name()
	require.NoError(t, err)

	t.Logf("name: %v", n)
}

func Test_trimURL(t *testing.T) {
	type args struct {
		fetchURL string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "git@github.com:ovh/cds.git",
			args:    args{"git@github.com:ovh/cds.git"},
			want:    "ovh/cds",
			wantErr: false,
		},
		{
			name:    "ssh://git@my.gitserver.net:7999/ovh/cds.git",
			args:    args{"ssh://git@my.gitserver.net:7999/ovh/cds.git"},
			want:    "ovh/cds",
			wantErr: false,
		},
		{
			name:    "https://github.com/ovh/cds",
			args:    args{"https://github.com/ovh/cds"},
			want:    "ovh/cds",
			wantErr: false,
		},
		{
			name:    "https://francois.samin@my.gitserver.net/scm/ovh/cds.git",
			args:    args{"https://francois.samin@my.gitserver.net/scm/ovh/cds.git"},
			want:    "ovh/cds",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := trimURL(tt.args.fetchURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("trimURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("trimURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalConfigGet(t *testing.T) {
	r, err := New(".")
	require.NoError(t, err)

	require.NoError(t, r.LocalConfigSet("foo", "bar", "value"))

	val, err := r.LocalConfigGet("foo", "bar")
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestLatestCommit(t *testing.T) {
	r, err := New(".")
	require.NoError(t, err)

	c, err := r.LatestCommit()
	t.Logf("%+v", c)
	require.NoError(t, err)
}

func TestDefaultBranch(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	s, err := r.DefaultBranch()
	require.NoError(t, err)
	assert.Equal(t, "master", s)
}

func TestGlob(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	files, err := r.Glob("**/*.md")
	require.NoError(t, err)
	var readmeFound, licenceFound bool
	for _, f := range files {
		switch f {
		case "LICENSE.md":
			licenceFound = true
		case "README.md":
			readmeFound = true
		}
	}
	assert.True(t, readmeFound, "README.md not found")
	assert.True(t, licenceFound, "LICENSE.md not found")
}

func TestOpen(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	files, err := r.Glob("**/*.md")
	require.NoError(t, err)
	f, err := r.Open(files[0])
	require.NoError(t, err)
	assert.NotNil(t, f)
	if err != nil {
		f.Close()
	}
}

func TestCheckoutNewBranch_Checkout_DeleteBranch(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))
	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)

	require.NoError(t, r.CheckoutNewBranch("newBranch"))
	require.NoError(t, r.Checkout("master"))
	require.NoError(t, r.DeleteBranch("newBranch"))
}

func TestPushError(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	r, err := Clone(path, "https://github.com/fsamin/go-repo.git", WithHTTPAuth("user", "mypassword"))
	require.NoError(t, err)

	require.NoError(t, r.CheckoutNewBranch("TestBranch"))
	require.NoError(t, r.Write("README.md", strings.NewReader("this is a test")))
	require.NoError(t, r.Add("README.md"))
	require.NoError(t, r.Commit("This is a test"))

	errPush := r.Push("origin", "TestBranch")
	assert.Error(t, errPush)
	assert.Contains(t, errPush.Error(), "https://github.com/fsamin/go-repo.git")
	assert.NotContains(t, errPush.Error(), "mypassword")
}

func TestCheckCommit(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	r, err := Clone(path, "https://github.com/fsamin/go-repo.git", WithHTTPAuth("user", "mypassword"))
	require.NoError(t, err)

	require.NoError(t, r.CheckoutNewBranch("TestBranch"))
	require.NoError(t, r.Write("README2.md", strings.NewReader("this is a test")))
	require.NoError(t, r.Add("README2.md"))
	require.NoError(t, r.Commit("This is a test"))
	c, err := r.LatestCommit()
	require.NoError(t, err)

	_, err = r.GetCommit(c.Hash)
	require.NoError(t, err)
}

func TestPush(t *testing.T) {
	if os.Getenv("TRAVIS_BUILD_DIR") != "" {
		t.SkipNow()
	}
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	privateKey, err := ioutil.ReadFile("travis_id_rsa")
	if err != nil {
		t.SkipNow()
	}

	r, err := Clone(path, "git@github.com:fsamin/go-repo.git", WithSSHAuth(privateKey), WithUser("francois.samin+github@gmail.com", "fsamin"))
	require.NoError(t, err)

	require.NoError(t, r.CheckoutNewBranch("TestBranch"))
	require.NoError(t, r.Write("README.md", strings.NewReader("this is a test")))
	require.NoError(t, r.Add("README.md"))
	require.NoError(t, r.Commit("This is a test"))
	require.NoError(t, r.Push("origin", "TestBranch"))
}

func TestHasDiverged(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)
	hasDiverged, err := r.HasDiverged()
	require.NoError(t, err)
	assert.False(t, hasDiverged)
}

func TestRemove(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)
	require.NoError(t, r.Remove("cmd.go"))

	status, err := r.Status()
	require.NoError(t, err)
	t.Log(status)
	assert.True(t, strings.Contains(status, "D"))
}

func TestCommitWithUser(t *testing.T) {
	path := filepath.Join(os.TempDir(), "testdata", t.Name())
	defer os.RemoveAll(path)

	require.NoError(t, os.MkdirAll(path, os.FileMode(0755)))

	r, err := Clone(path, "https://github.com/fsamin/go-repo.git")
	require.NoError(t, err)
	require.NoError(t, r.Write("README.md", strings.NewReader("this is a test")))
	require.NoError(t, r.Add("README.md"))
	require.NoError(t, r.Commit("This is a test", WithUser("foo@bar.com", "foo.bar")))
	commit, err := r.LatestCommit()
	require.NoError(t, err)
	assert.Equal(t, "foo.bar", commit.Author)
}
