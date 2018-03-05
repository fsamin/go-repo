package repo

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	zglob "github.com/mattn/go-zglob"
)

func Clone(path, url string, opts ...RepoOption) (Repo, error) {
	r := Repo{path: path}
	for _, f := range opts {
		if err := f(&r); err != nil {
			return r, err
		}
	}
	_, err := r.runCmd("git", "clone", url, ".")
	if err != nil {
		return r, err
	}
	return r, nil
}

func New(path string, opts ...RepoOption) (Repo, error) {
	r := Repo{path: path}
	for _, f := range opts {
		if err := f(&r); err != nil {
			return r, err
		}
	}
	dotGit := filepath.Join(path, ".git")
	if _, err := os.Stat(dotGit); err != nil || os.IsNotExist(err) {
		return r, err
	}
	return r, nil
}

func (r Repo) FetchURL() (string, error) {
	stdOut, err := r.runCmd("git", "remote", "show", "origin", "-n")
	if err != nil {
		return "", err
	}

	reader := bufio.NewReader(strings.NewReader(stdOut))
	var fetchURL string
	for {
		b, _, err := reader.ReadLine()
		if err == io.EOF || b == nil {
			break
		}
		if err != nil {
			return "", err
		}
		s := string(b)
		if strings.Contains(s, "Fetch URL:") {
			fetchURL = strings.Replace(s, "Fetch URL:", "", 1)
			fetchURL = strings.TrimSpace(fetchURL)
		}
	}

	return fetchURL, nil
}

func (r Repo) Name() (string, error) {
	fetchURL, err := r.FetchURL()
	if err != nil {
		return "", err
	}

	return trimURL(fetchURL)
}

func (r Repo) LocalConfigGet(section, key string) (string, error) {
	s, err := r.runCmd("git", "config", "--local", "--get", fmt.Sprintf("%s.%s", section, key))
	if err != nil {
		return "", err
	}
	return s[:len(s)-1], nil
}

func (r Repo) LocalConfigSet(section, key, value string) error {
	conf, _ := r.LocalConfigGet(section, key)
	s := fmt.Sprintf("%s.%s", section, key)
	if conf != "" {
		if _, err := r.runCmd("git", "config", "--local", "--unset", s); err != nil {
			return err
		}
	}

	if _, err := r.runCmd("git", "config", "--local", "--add", s, value); err != nil {
		return err
	}

	return nil
}

func trimURL(fetchURL string) (string, error) {
	repoName := fetchURL

	if strings.HasSuffix(repoName, ".git") {
		repoName = repoName[:len(repoName)-4]
	}

	if strings.HasPrefix(repoName, "https://") {
		repoName = repoName[8:]
		for strings.Count(repoName, "/") > 1 {
			firstSlash := strings.Index(repoName, "/")
			if firstSlash == -1 {
				return "", fmt.Errorf("invalid url")
			}
			repoName = repoName[firstSlash+1:]
		}
		return repoName, nil
	}

	if strings.HasPrefix(repoName, "ssh://") {
		// ssh://[user@]server/project.git
		repoName = repoName[6:]
		firstSlash := strings.Index(repoName, "/")
		if firstSlash == -1 {
			return "", fmt.Errorf("invalid url")
		}
		repoName = repoName[firstSlash+1:]
	} else {
		// [user@]server:project.git
		firstSemicolon := strings.Index(repoName, ":")
		if firstSemicolon == -1 {
			return "", fmt.Errorf("invalid url")
		}
		repoName = repoName[firstSemicolon+1:]
	}

	return repoName, nil
}

func (r Repo) LatestCommit() (Commit, error) {
	c := Commit{}
	hash, err := r.runCmd("git", "rev-parse", "HEAD")
	if err != nil {
		return c, err
	}

	details, err := r.runCmd("git", "show", hash[:7], "--quiet", "--pretty=%at||%an||%s||%b")
	if err != nil {
		return c, err
	}

	c.LongHash = hash[:len(hash)-1]
	c.Hash = hash[:7]

	splittedDetails := strings.SplitN(details, "||", 4)

	ts, err := strconv.ParseInt(splittedDetails[0], 10, 64)
	if err != nil {
		return c, err
	}
	c.Date = time.Unix(ts, 0)
	c.Author = splittedDetails[1]
	c.Subject = splittedDetails[2]
	c.Body = splittedDetails[3]

	return c, nil
}

func (r Repo) CurrentBranch() (string, error) {
	b, err := r.runCmd("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return b[:len(b)-1], nil
}

func (r Repo) FetchRemoteBranch(remote, branch string) error {
	if _, err := r.runCmd("git", "fetch"); err != nil {
		return fmt.Errorf("unable to git fetch: %s", err)
	}
	_, err := r.runCmd("git", "checkout", "-b", branch, "--track", remote+"/"+branch)
	if err != nil {
		return fmt.Errorf("unable to git checkout: %s", err)
	}
	return nil
}

func (r Repo) Pull(remote, branch string) error {
	_, err := r.runCmd("git", "pull", remote, branch)
	return err
}

func (r Repo) ResetHard(hash string) error {
	_, err := r.runCmd("git", "reset", "--hard", hash)
	return err
}

func (r Repo) DefaultBranch() (string, error) {
	s, err := r.runCmd("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	s = strings.Replace(s, "\n", "", 1)
	s = strings.Replace(s, "refs/remotes/origin/", "", 1)
	return s, nil
}

func (r Repo) Glob(s string) ([]string, error) {
	p := filepath.Join(r.path, s)
	files, err := zglob.Glob(p)
	if err != nil {
		return nil, err
	}
	for i, f := range files {
		files[i], err = filepath.Rel(r.path, f)
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func (r Repo) Open(s string) (*os.File, error) {
	p := filepath.Join(r.path, s)
	return os.Open(p)
}

type RepoOption func(r *Repo) error

func WithSSHAuth(privateKey []byte) RepoOption {
	return func(r *Repo) error {
		r.sshKey = &sshKey{
			content: privateKey,
		}

		h := md5.New()
		if _, err := io.WriteString(h, string(privateKey)); err != nil {
			return err
		}

		u, err := user.Current()
		if err != nil {
			return err
		}

		md5sum := fmt.Sprintf("%x", h.Sum(nil))
		dir := filepath.Join(u.HomeDir, ".lib-git-repo", md5sum)
		if err := os.MkdirAll(dir, os.FileMode(0700)); err != nil {
			return err
		}
		r.sshKey.filename = filepath.Join(dir, "id_rsa")
		return ioutil.WriteFile(r.sshKey.filename, r.sshKey.content, os.FileMode(0600))
	}
}

func WithHTTPAuth(username string, password string) RepoOption {
	return func(r *Repo) error {
		r.httpUsername = username
		r.httpPassword = password
		return nil
	}
}

func InstallPGPKey(privateKey []byte) RepoOption {
	return func(r *Repo) error {
		return nil
	}
}

func WithVerbose() RepoOption {
	return func(r *Repo) error {
		r.verbose = true
		return nil
	}
}
