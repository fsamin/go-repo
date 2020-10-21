package repo

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CloneBare a git bare repository from the specified url to the destination path. Use Options to force the use of SSH Key and or PGP Key on this repo
func CloneBare(ctx context.Context, path, url string, opts ...Option) (Repo, error) {
	r := Repo{path: path, url: url}
	for _, f := range opts {
		if err := f(ctx, &r); err != nil {
			return r, err
		}
	}
	if r.verbose {
		r.log("Cloning %s\n", r.url)
	}
	_, err := r.runCmd(ctx, "git", "clone", "--bare", r.url, ".")
	if err != nil {
		return r, err
	}
	return r, nil
}

// NewBare instanciance a bare repo instance from the path assuming the repo has already been cloned in.
func NewBare(ctx context.Context, path string, opts ...Option) (b BareRepo, err error) {
	b = BareRepo{Repo{path: path}}
	b.repo.path, err = findRefsDirectory(path)
	if err != nil {
		return b, err
	}

	output, _ := b.repo.runCmd(ctx, "git", "rev-parse", "--is-bare-repository")
	if !strings.Contains(output, "true") {
		return b, errors.New("path is not a bare repository")
	}

	for _, f := range opts {
		if err := f(ctx, &b.repo); err != nil {
			return b, err
		}
	}

	return b, nil
}

func findRefsDirectory(p string) (string, error) {
	p = path.Join(p)
	p, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	if p == string(filepath.Separator) {
		return "", errors.New("refs directory not found")
	}

	if checkRefsDirectory(p) {
		return p, nil
	}

	parent := filepath.Dir(p)
	return findRefsDirectory(parent)
}

func checkRefsDirectory(path string) bool {
	dotGit := filepath.Join(path, "refs")
	if _, err := os.Stat(dotGit); err != nil || os.IsNotExist(err) {
		return false
	}
	return true
}

func (b BareRepo) ListFiles(ctx context.Context) ([]string, error) {
	output, err := b.repo.runCmd(ctx, "git", "ls-tree", "--full-tree", "--name-only", "-r", "HEAD")
	if err != nil {
		return nil, err
	}
	output = strings.TrimSpace(output)
	files := strings.Split(output, "\n")
	return files, nil
}

var singleSpacePattern = regexp.MustCompile(`\s+`)

func (b BareRepo) FileSize(ctx context.Context, filename string) (int64, error) {

	output, err := b.repo.runCmd(ctx, "git", "ls-tree", "--full-tree", "--long", "-r", "HEAD")
	if err != nil {
		return -1, err
	}
	output = strings.TrimSpace(output)
	files := strings.Split(output, "\n")
	for _, file := range files {
		file = strings.Replace(file, "\t", " ", -1)
		file = singleSpacePattern.ReplaceAllString(file, " ")
		tuple := strings.SplitN(file, " ", 5)
		if len(tuple) != 5 {
			return -1, errors.New("unable to file size: " + file)
		}
		if tuple[4] == filename {
			return strconv.ParseInt(tuple[3], 10, 64)
		}
	}
	return -1, errors.New("unable to file size: file " + filename + " not found")
}

func (b BareRepo) ReadFile(ctx context.Context, filename string) (io.Reader, error) {
	output, err := b.repo.runCmd(ctx, "git", "show", "HEAD:"+filename)
	if err != nil {
		return nil, err
	}
	return strings.NewReader(output), nil
}

func (b BareRepo) FetchURL(ctx context.Context) (string, error) {
	return b.repo.FetchURL(ctx)
}

func (b BareRepo) Name(ctx context.Context) (string, error) {
	return b.repo.Name(ctx)
}

func (b BareRepo) Path() string {
	return b.repo.path
}

func (b BareRepo) CommitsBetween(ctx context.Context, from, to time.Time, branch string) ([]Commit, error) {
	return b.repo.CommitsBetween(ctx, from, to, branch)
}

func (b BareRepo) DefaultBranch(ctx context.Context) (string, error) {
	return b.repo.DefaultBranch(ctx)
}
