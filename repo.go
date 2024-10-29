package repo

import (
	"bufio"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
	zglob "github.com/mattn/go-zglob"
	"github.com/pkg/errors"
)

var urlRegExp = regexp.MustCompile(`https:\/\/[-a-zA-Z0-9@:%._\+#?&//=]*`)

// Clone a git repository from the specified url to the destination path. Use Options to force the use of SSH Key and or PGP Key on this repo
func Clone(ctx context.Context, path, url string, opts ...Option) (Repo, error) {
	r := Repo{path: path, url: url}
	for _, f := range opts {
		if err := f(ctx, &r); err != nil {
			return r, err
		}
	}
	if r.verbose {
		r.log("Cloning %s\n", r.url)
	}
	_, err := r.runCmd(ctx, "git", "clone", r.url, ".")
	if err != nil {
		return r, err
	}
	return r, nil
}

// New instanciance a repo instance from the path assuming the repo has already been cloned in.
func New(ctx context.Context, path string, opts ...Option) (r Repo, err error) {
	r = Repo{path: path}
	r.path, err = findDotGitDirectory(path)
	if err != nil {
		return r, err
	}

	for _, f := range opts {
		if err := f(ctx, &r); err != nil {
			return r, err
		}
	}

	return r, nil
}

var windowsPathRegex = regexp.MustCompile(`^[a-zA-Z]:\\$`)

func pathIsRoot(p string) bool {
	if runtime.GOOS == "windows" {
		return windowsPathRegex.MatchString(p)
	}
	return p == string(filepath.Separator)
}

func checkDotGitDirectory(path string) bool {
	dotGit := filepath.Join(path, ".git")
	if _, err := os.Stat(dotGit); err != nil || os.IsNotExist(err) {
		return false
	}
	return true
}

func findDotGitDirectory(p string) (string, error) {
	p = path.Join(p)
	p, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	if pathIsRoot(p) {
		return "", errors.New(".git directory not found")
	}

	if checkDotGitDirectory(p) {
		return p, nil
	}

	parent := filepath.Dir(p)
	return findDotGitDirectory(parent)
}

// FetchURL returns the git URL the the remote origin
func (r Repo) FetchURL(ctx context.Context) (string, error) {
	stdOut, err := r.runCmd(ctx, "git", "remote", "show", "origin", "-n")
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

// Name returns the name of the repo, deduced from the remote origin URL
func (r Repo) Name(ctx context.Context) (string, error) {
	fetchURL, err := r.FetchURL(ctx)
	if err != nil {
		return "", err
	}

	return trimURL(fetchURL)
}

// LocalConfigGet returns data from the local git config
func (r Repo) LocalConfigGet(ctx context.Context, section, key string) (string, error) {
	s, err := r.runCmd(ctx, "git", "config", "--local", "--get", fmt.Sprintf("%s.%s", section, key))
	if err != nil {
		return "", err
	}
	return s[:len(s)-1], nil
}

// LocalConfigSet set data in the local git config
func (r Repo) LocalConfigSet(ctx context.Context, section, key, value string) error {
	conf, _ := r.LocalConfigGet(ctx, section, key)
	s := fmt.Sprintf("%s.%s", section, key)
	if conf != "" {
		if _, err := r.runCmd(ctx, "git", "config", "--local", "--unset-all", s); err != nil {
			return err
		}
	}

	if _, err := r.runCmd(ctx, "git", "config", "--local", "--add", s, value); err != nil {
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

// Commits returns all the commit between
func (r Repo) Commits(ctx context.Context, from, to string) ([]Commit, error) {
	if from == "0000000000000000000000000000000000000000" {
		from = ""
	}
	if from != "" {
		from = from + ".."
	}
	s, err := r.runCmd(ctx, "git", "rev-list", from+to)
	if err != nil {
		return nil, err
	}
	var commitsString []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		s := scanner.Text()
		commitsString = append(commitsString, s)
	}
	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for _, c := range commitsString {
		comm, err := r.GetCommit(ctx, c, CommitOption{DisableDiffDetail: true})
		if err != nil {
			return nil, err
		}
		commits = append(commits, comm)
	}

	return commits, err
}

func (r Repo) CommitsBetween(ctx context.Context, from, to time.Time, branch string) ([]Commit, error) {
	s, err := r.runCmd(ctx, "git", "log", branch, "--since", from.Format("2006-01-02"), "--until", to.Format("2006-01-02"), "--pretty=%H")
	if err != nil {
		return nil, err
	}

	var commitsString []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		s := scanner.Text()
		commitsString = append(commitsString, s)
	}
	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for _, c := range commitsString {
		comm, err := r.GetCommit(ctx, c, CommitOption{DisableDiffDetail: true})
		if err != nil {
			return nil, err
		}
		commits = append(commits, comm)
	}

	return commits, nil
}

func (r Repo) parseDiff(ctx context.Context, hash, diff string, opt CommitOption) (map[string]File, error) {
	Files := make(map[string]File)

	// Read line per line the last item
	scanner := bufio.NewScanner(strings.NewReader(diff))
	for scanner.Scan() {
		s := scanner.Text()
		var tuple []string
		if strings.Contains(s, "\t") {
			tuple = strings.SplitN(s, "\t", 2)
		}
		filename := strings.TrimSpace(tuple[1])
		status := strings.TrimSpace(tuple[0])
		diff, err := r.Diff(ctx, hash, filename)
		if err != nil {
			return nil, fmt.Errorf("unable to compute diff on file %s for commit %s: %v", filename, hash, err)
		}

		f := File{
			Filename: filename,
			Status:   status,
			Diff:     diff,
		}

		//Scan the diff output
		if !opt.DisableDiffDetail {
			diffScanner := bufio.NewScanner(strings.NewReader(diff))
			diffScanner.Buffer(nil, 1024*1024)
			var currentHunk *Hunk
			for diffScanner.Scan() {
				line := diffScanner.Text()
				switch {
				case strings.HasPrefix(line, "@@ "):
					line := strings.TrimPrefix(line, "@@ ")
					if currentHunk != nil {
						f.DiffDetail.Hunks = append(f.DiffDetail.Hunks, *currentHunk)
						currentHunk = nil
					}
					currentHunk = new(Hunk)
					currentHunk.Header = strings.TrimSpace(strings.Split(line, "@@")[0])
					currentHunk.Content = strings.Join(strings.Split(line, "@@")[1:], "")
				case currentHunk != nil && strings.HasPrefix(line, "-"):
					currentHunk.RemovedLines = append(currentHunk.RemovedLines, strings.TrimPrefix(line, "-"))
					currentHunk.Content += "\n" + line
				case currentHunk != nil && strings.HasPrefix(line, "+"):
					currentHunk.AddedLines = append(currentHunk.AddedLines, strings.TrimPrefix(line, "+"))
					currentHunk.Content += "\n" + line
				case currentHunk != nil:
					currentHunk.Content += "\n" + line
				}
			}

			if currentHunk != nil {
				f.DiffDetail.Hunks = append(f.DiffDetail.Hunks, *currentHunk)
			}

			err = diffScanner.Err()
			if err != nil {
				return nil, fmt.Errorf("unable to compute diff on file %s for commit %s: %v", filename, hash, err)
			}
		}
		Files[filename] = f
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return Files, nil
}

// GetTag returns a tag
func (r Repo) GetTag(ctx context.Context, tagName string) (Tag, error) {
	tagName = strings.TrimFunc(tagName, func(r rune) bool {
		return r == '\n' || r == ' ' || r == '\t'
	})
	t := Tag{}
	details, err := r.runCmd(ctx, "git", "show", tagName, "--pretty=||%at||%an||%ae||%s||%b||%H||%GK||", "--name-status")
	if err != nil {
		return t, err
	}

	splittedDetails := strings.SplitN(details, "||", 9)
	splittedDetails = splittedDetails[1:]

	ts, err := strconv.ParseInt(splittedDetails[0], 10, 64)
	if err != nil {
		return t, err
	}
	t.Date = time.Unix(ts, 0)
	t.Author = splittedDetails[1]
	t.AuthorEmail = splittedDetails[2]
	t.Subject = splittedDetails[3]
	t.Body = splittedDetails[4]
	t.LongHash = splittedDetails[5]
	t.Hash = t.LongHash[:7]
	t.GPGKeyID = splittedDetails[6]
	return t, err
}

// GetCommit returns a commit
func (r Repo) GetCommit(ctx context.Context, hash string, opts CommitOption) (Commit, error) {
	hash = strings.TrimFunc(hash, func(r rune) bool {
		return r == '\n' || r == ' ' || r == '\t'
	})
	c := Commit{}
	details, err := r.runCmd(ctx, "git", "show", hash, "--pretty=%at||%an||%ae||%s||%b||%GK||", "--name-status")
	if err != nil {
		return c, err
	}

	c.LongHash = hash
	c.Hash = hash[:7]

	splittedDetails := strings.SplitN(details, "||", 7)

	ts, err := strconv.ParseInt(splittedDetails[0], 10, 64)
	if err != nil {
		return c, err
	}
	c.Date = time.Unix(ts, 0)
	c.Author = splittedDetails[1]
	c.AuthorEmail = splittedDetails[2]
	c.Subject = splittedDetails[3]
	c.Body = splittedDetails[4]
	c.GPGKeyID = splittedDetails[5]

	fileList := strings.TrimSpace(splittedDetails[6])
	c.Files, err = r.parseDiff(ctx, hash, fileList, opts)
	return c, err
}

func (r Repo) DiffSinceCommitMergeBase(ctx context.Context, hash string) (map[string]File, error) {
	details, err := r.runCmd(ctx, "git", "diff", hash, "--name-status", "--merge-base")
	if err != nil {
		return nil, err
	}

	result := make(map[string]File)

	splittedFiles := strings.Split(details, "\n")
	for _, fLine := range splittedFiles {
		if len(strings.Trim(fLine, " ")) == 0 {
			continue
		}
		fileData := strings.Split(fLine, "\t")
		f := File{
			Status:   fileData[0],
			Filename: fileData[1],
		}
		result[f.Filename] = f
	}
	return result, nil
}

func (r Repo) DiffSinceCommit(ctx context.Context, hash string) (map[string]File, error) {
	details, err := r.runCmd(ctx, "git", "diff", hash, "--name-status")
	if err != nil {
		return nil, err
	}

	result := make(map[string]File)

	splittedFiles := strings.Split(details, "\n")
	for _, fLine := range splittedFiles {
		if len(strings.Trim(fLine, " ")) == 0 {
			continue
		}
		fileData := strings.Split(fLine, "\t")
		f := File{
			Status:   fileData[0],
			Filename: fileData[1],
		}
		result[f.Filename] = f
	}
	return result, nil
}

func (r Repo) Diff(ctx context.Context, hash string, filename string) (string, error) {
	if hash == "" {
		return r.runCmd(ctx, "git", "diff", "--pretty=", "--", filename)
	}
	return r.runCmd(ctx, "git", "show", hash, "--pretty=", "--", filename)
}

// ExistsDiff returns true if there are no commited diff in the repo.
func (r Repo) ExistsDiff(ctx context.Context) bool {
	if _, err := r.runCmd(ctx, "git", "diff", "--quiet", "HEAD", "--"); err != nil {
		return true
	}
	return false
}

// LatestCommit returns the latest commit of the current branch
func (r Repo) LatestCommit(ctx context.Context, opts CommitOption) (Commit, error) {
	c := Commit{}
	hash, err := r.runCmd(ctx, "git", "rev-parse", "HEAD")
	if err != nil {
		return c, err
	}
	return r.GetCommit(ctx, hash, opts)
}

// CurrentBranch returns the current branch
func (r Repo) CurrentBranch(ctx context.Context) (string, error) {
	b, err := r.runCmd(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return b[:len(b)-1], nil
}

func (r Repo) VerifyCommit(ctx context.Context, commit string) error {
	_, err := r.runCmd(ctx, "git", "verify-commit", commit)
	if err != nil {
		return fmt.Errorf("commit not verify: %v", err)
	}
	return nil
}

// VerifyTag returns the sha1 of the tag if exists, if it doesn't exist, it returns an error
func (r Repo) VerifyTag(ctx context.Context, tag string) (string, error) {
	sha1, err := r.runCmd(ctx, "git", "rev-parse", "--verify", tag)
	if err != nil {
		return "", fmt.Errorf("tag not found: %v", err)
	}
	return sha1[:len(sha1)-1], nil
}

// FetchRemoteTags fetch all tags
func (r Repo) FetchRemoteTags(ctx context.Context, remote string) error {
	// Get tags from remote
	if _, err := r.runCmd(ctx, "git", "fetch", "--tags", "--force", remote); err != nil {
		return fmt.Errorf("unable to git fetch tags: %s", err)
	}

	return nil
}

// FetchRemoteTag deletes given tag if exists, then fetch new tags and checkout given tag.
func (r Repo) FetchRemoteTag(ctx context.Context, remote, tag string) error {
	// delete tag if exist
	if _, err := r.runCmd(ctx, "git", "rev-parse", "--verify", tag); err == nil {
		if _, err := r.runCmd(ctx, "git", "tag", "-d", tag); err != nil {
			return fmt.Errorf("unable to git delete tag: %s", err)
		}
	}

	// Get tag from remote
	if _, err := r.runCmd(ctx, "git", "fetch", "--tags", "--force", remote); err != nil {
		return fmt.Errorf("unable to git fetch tags: %s", err)
	}

	if _, err := r.runCmd(ctx, "git", "checkout", tag); err != nil {
		return fmt.Errorf("unable to git checkout: %s", err)
	}
	return nil
}

// LocalBranchExists returns if given branch exists locally and has upstream.
func (r Repo) LocalBranchExists(ctx context.Context, branch string) (exists, hasUpstream bool) {
	if _, err := r.runCmd(ctx, "git", "rev-parse", "--verify", branch); err == nil {
		exists = true
	}
	if _, err := r.runCmd(ctx, "git", "rev-parse", "--abbrev-ref", branch+"@{upstream}"); err == nil {
		hasUpstream = true
	}
	return
}

// FetchRemoteBranch runs a git fetch then checkout the remote branch
func (r Repo) FetchRemoteBranch(ctx context.Context, remote, branch string) error {
	if _, err := r.runCmd(ctx, "git", "fetch", remote); err != nil {
		return fmt.Errorf("unable to git fetch: %s", err)
	}

	branchExist, hasUpstream := r.LocalBranchExists(ctx, branch)
	if branchExist {
		if hasUpstream {
			_, err := r.runCmd(ctx, "git", "checkout", branch)
			if err != nil {
				return fmt.Errorf("unable to git checkout: %s", err)
			}
			return nil
		}
		// the branch exist but has no upstream. Delete it
		if _, err := r.runCmd(ctx, "git", "branch", "-d", branch); err != nil {
			return fmt.Errorf("unable to git delete: %s", err)
		}
	}

	_, err := r.runCmd(ctx, "git", "checkout", "-b", branch, "--track", remote+"/"+branch)
	if err != nil {
		return fmt.Errorf("unable to git checkout new branch: %s", err)
	}
	return nil
}

// Checkout checkouts a branch on the local repository
func (r Repo) Checkout(ctx context.Context, branch string) error {
	_, err := r.runCmd(ctx, "git", "checkout", branch)
	if err != nil {
		return fmt.Errorf("unable to git checkout: %s", err)
	}
	return nil
}

// CheckoutNewBranch checkouts a new branch on the local repository
func (r Repo) CheckoutNewBranch(ctx context.Context, branch string) error {
	_, err := r.runCmd(ctx, "git", "checkout", "-b", branch)
	if err != nil {
		return fmt.Errorf("unable to git checkout: %s", err)
	}
	return nil
}

// DeleteBranch deletes a branch on the local repository
func (r Repo) DeleteBranch(ctx context.Context, branch string) error {
	_, err := r.runCmd(ctx, "git", "branch", "-d", branch)
	if err != nil {
		return fmt.Errorf("unable to delete branch: %s", err)
	}
	return nil
}

// Pull pulls a branch from a remote
func (r Repo) Pull(ctx context.Context, remote, branch string) error {
	_, err := r.runCmd(ctx, "git", "pull", remote, branch)
	return err
}

// ResetHard hard resets a ref
func (r Repo) ResetHard(ctx context.Context, hash string) error {
	_, err := r.runCmd(ctx, "git", "reset", "--hard", hash)
	return err
}

// DefaultBranch returns the default branch of the remote origin
func (r Repo) DefaultBranch(ctx context.Context) (string, error) {
	details, err := r.runCmd(ctx, "git", "remote", "show", "origin")
	if err != nil {
		return "", err
	}
	var defaultBranch string
	splitted := strings.Split(details, "\n")
	for _, l := range splitted {
		if strings.Contains(l, "HEAD branch:") {
			defaultBranch = strings.TrimSpace(strings.Replace(l, "HEAD branch:", "", 1))
			return defaultBranch, nil
		}
	}
	return "", fmt.Errorf("no default branch found")
}

// Glob returns the matching files in the repo
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

// Open opens a file from the repo
func (r Repo) Open(s string) (*os.File, error) {
	p := filepath.Join(r.path, s)
	return os.Open(p)
}

// Write writes a file in the repo
func (r Repo) Write(s string, content io.Reader) error {
	p := filepath.Join(r.path, s)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, os.FileMode(0644))
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, content); err != nil {
		return err
	}
	return nil
}

// Add file contents to the index
func (r Repo) Add(ctx context.Context, s ...string) error {
	args := append([]string{"add"}, s...)
	out, err := r.runCmd(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("command 'git add' failed: %v (%s)", err, out)
	}
	return nil
}

// Remove file or directory
func (r Repo) Remove(ctx context.Context, s ...string) error {
	args := append([]string{"rm", "-f", "-r"}, s...)
	out, err := r.runCmd(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("command 'git rm' failed: %v (%s)", err, out)
	}
	return nil
}

// Commit the index
func (r Repo) Commit(ctx context.Context, m string, opts ...Option) error {
	for _, f := range opts {
		if err := f(ctx, &r); err != nil {
			return err
		}
	}
	out, err := r.runCmd(ctx, "git", "commit", "-m", strconv.Quote(m))
	if err != nil {
		return fmt.Errorf("command 'git commit' failed: %v (%s)", err, out)
	}
	return nil
}

// PushTags (always with force) the branch
func (r Repo) PushTags(ctx context.Context, remote string, opts ...Option) error {
	for _, f := range opts {
		if err := f(ctx, &r); err != nil {
			return err
		}
	}
	out, err := r.runCmd(ctx, "git", "push", remote, "--tags")
	if err != nil {
		errS := fmt.Sprintf("%v", err)
		URLS := urlRegExp.FindString(errS)
		URL, errURL := url.Parse(URLS)
		if errURL == nil {
			URL.User = nil
			errS = strings.Replace(errS, URLS, URL.String(), -1)
		}
		return fmt.Errorf("%s (%s)", errS, out)
	}
	return nil
}

// Push (always with force) the branch
func (r Repo) Push(ctx context.Context, remote, branch string, opts ...Option) error {
	for _, f := range opts {
		if err := f(ctx, &r); err != nil {
			return err
		}
	}
	out, err := r.runCmd(ctx, "git", "push", "-f", "-u", remote, branch)
	if err != nil {
		errS := fmt.Sprintf("%v", err)
		URLS := urlRegExp.FindString(errS)
		URL, errURL := url.Parse(URLS)
		if errURL == nil {
			URL.User = nil
			errS = strings.Replace(errS, URLS, URL.String(), -1)
		}
		return fmt.Errorf("%s (%s)", errS, out)
	}
	return nil
}

// RemoteAdd run git remote add
func (r Repo) RemoteAdd(ctx context.Context, remote, branch, url string) error {
	var args []string
	if branch != "" {
		args = []string{"remote", "add", "-t", branch, remote, url}
	} else {
		args = []string{"remote", "add", remote, url}
	}
	out, err := r.runCmd(ctx, "git", args...)
	if err != nil {
		return fmt.Errorf("command 'git remote add' failed: %v (%s)", err, out)
	}
	return nil
}

// RemoteShow run git remote show
func (r Repo) RemoteShow(ctx context.Context, remote string) (string, error) {
	args := []string{"remote", "show", remote}
	out, err := r.runCmd(ctx, "git", args...)
	if err != nil {
		return out, fmt.Errorf("command 'git remote show' failed: %v (%s)", err, out)
	}
	return out, nil
}

// Status run the git status command
func (r Repo) Status(ctx context.Context) (string, error) {
	args := []string{"status", "-s", "-uall"}
	out, err := r.runCmd(ctx, "git", args...)
	if err != nil {
		return "", fmt.Errorf("command 'git status' failed: %v (%s)", err, out)
	}
	return out, nil
}

func (r Repo) CurrentSnapshot(ctx context.Context, opts CommitOption) (map[string]File, error) {
	diffFileList, err := r.Status(ctx)
	if err != nil {
		return nil, err
	}
	return r.parseDiff(ctx, "", diffFileList, opts)
}

func (r Repo) HasDiverged(ctx context.Context) (bool, error) {
	out, err := r.runCmd(ctx, "git", "status")
	if err != nil {
		return false, fmt.Errorf("command 'git status' failed: %v (%s)", err, out)
	}
	if strings.Contains(out, "diverged") {
		return true, nil
	}
	return false, nil
}

func (r Repo) HookList() ([]string, error) {
	hooksPath := filepath.Join(r.path, ".git", "hooks")
	var files []string
	err := filepath.Walk(hooksPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && !strings.HasSuffix(path, ".sample") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (r Repo) DeleteHook(name string) error {
	hookPath := filepath.Join(r.path, ".git", "hooks", name)
	return os.Remove(hookPath)
}

func (r Repo) WriteHook(name string, content []byte) error {
	hookPath := filepath.Join(r.path, ".git", "hooks", name)
	return os.WriteFile(hookPath, content, os.FileMode(0755))
}

// Option is a function option
type Option func(ctx context.Context, r *Repo) error

// WithUser configure the git command to use user
func WithUser(email, name string) Option {
	return func(ctx context.Context, r *Repo) error {
		out, err := r.runCmd(ctx, "git", "config", "user.email", email)
		if err != nil {
			return fmt.Errorf("command 'git config user.email' failed: %v (%s)", err, out)
		}
		out, err = r.runCmd(ctx, "git", "config", "user.name", name)
		if err != nil {
			return fmt.Errorf("command 'git config user.name' failed: %v (%s)", err, out)
		}
		return err
	}
}

func WithSignKey(keyId string) Option {
	return func(ctx context.Context, r *Repo) error {
		out, err := r.runCmd(ctx, "git", "config", "user.signingkey", keyId)
		if err != nil {
			return fmt.Errorf("command 'git config user.signingkey' failed: %v (%s)", err, out)
		}
		return nil
	}
}

// WithSSHAuth configure the git command to use a specific private key
func WithSSHAuth(privateKey []byte) Option {
	return func(_ context.Context, r *Repo) error {
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
		return os.WriteFile(r.sshKey.filename, r.sshKey.content, os.FileMode(0600))
	}
}

// WithHTTPAuth override the repo configuration to use http auth
func WithHTTPAuth(username string, password string) Option {
	return func(_ context.Context, r *Repo) error {
		u, err := url.Parse(r.url)
		if err != nil {
			return err
		}
		u.User = url.UserPassword(username, password)
		r.url = u.String()
		return nil
	}
}

// InstallPGPKey install a pgp key in the repo configuration
func InstallPGPKey(privateKey []byte) Option {
	return func(ctx context.Context, r *Repo) error {
		return nil
	}
}

// WithVerbose add some logs
func WithVerbose(logger func(format string, i ...interface{})) Option {
	return func(_ context.Context, r *Repo) error {
		r.verbose = true
		r.logger = logger
		return nil
	}
}

func (r Repo) log(format string, i ...interface{}) {
	if r.logger != nil {
		r.logger(format, i...)
	}
}

func (r Repo) Tags(ctx context.Context) ([]Tag, error) {
	s, err := r.runCmd(ctx, "git", "show-ref", "--tags")
	if err != nil {
		return nil, err
	}

	var commitsString []string
	var tagString []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		s := scanner.Text()
		h := strings.Split(s, " ")[0]
		h = strings.TrimSpace(h)
		t := strings.Split(s, " ")[1]
		t = strings.TrimSpace(t)
		commitsString = append(commitsString, h)
		tagString = append(tagString, t)
	}
	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	var tags []Tag
	for i, c := range commitsString {
		t, err := r.GetCommit(ctx, c, CommitOption{DisableDiffDetail: true})
		if err != nil {
			return nil, err
		}
		tags = append(tags, Tag{Commit: t, Message: tagString[i]})
	}

	return tags, nil
}

type SubmoduleOpt struct {
	Init      bool
	Recursive bool
}

func (r Repo) SubmoduleUpdate(ctx context.Context, opt SubmoduleOpt) error {
	if r.verbose {
		r.log("Submodule update %v\n", r.url)
	}
	args := []string{"submodule", "update"}
	if opt.Init {
		args = append(args, "--init")
	}
	if opt.Recursive {
		args = append(args, "--recursive")
	}
	_, err := r.runCmd(ctx, "git", args...)
	if err != nil {
		return err
	}
	return nil
}

type DescribeOpt struct {
	DirtySemver      bool
	LongSemver       bool
	Long             bool
	RequireAnnotated bool
	Match            string
	DirtyMark        string
}

type Description struct {
	Dirty        bool            `json:"dirty"`
	Raw          string          `json:"raw"`
	Hash         string          `json:"hash"`
	Distance     int             `json:"distance"`
	Tag          string          `json:"tag"`
	Semver       *semver.Version `json:"semver"`
	Suffix       string          `json:"suffix"`
	SemverString string          `json:"semver_string"`
}

func (r Repo) Describe(ctx context.Context, opt *DescribeOpt) (*Description, error) {
	if opt == nil {
		opt = &DescribeOpt{
			DirtySemver: true,
			Long:        true,
			Match:       "v[0-9]*",
			DirtyMark:   "-dirty",
		}
	}
	args := []string{"describe", "--long", "--dirty=" + opt.DirtyMark, "--always"}
	if !opt.RequireAnnotated {
		args = append(args, "--tags")
	}
	if opt.Match != "" {
		args = append(args, "--match", opt.Match)
	}

	// git fetch --prune --unshallow
	_, _ = r.runCmd(ctx, "git", "fetch", "--prune", "--unshallow") // skip all errors

	// git fetch --tags
	if _, err := r.runCmd(ctx, "git", "fetch", "--tags"); err != nil {
		return nil, err
	}

	// git describe
	description, err := r.runCmd(ctx, "git", args...)
	if err != nil {
		return nil, err
	}

	description = strings.TrimSuffix(description, "\n")

	// description
	var output = &Description{
		Raw: description,
	}

	if strings.HasSuffix(description, opt.DirtyMark) {
		output.Dirty = true
		description = strings.TrimSuffix(description, opt.DirtyMark)
	}

	var tokens = strings.Split(description, "-")
	var suffixTokens []string

	tokens, output.Hash = slicePop(tokens)
	suffixTokens = append([]string{output.Hash}, suffixTokens...)
	if len(tokens) > 0 {
		var distanceS string
		tokens, distanceS = slicePop(tokens)
		output.Distance, _ = strconv.Atoi(distanceS)
		output.Tag = strings.Join(tokens, "-")
		output.Semver, _ = semver.NewVersion(output.Tag)
		output.SemverString = ""
		var build string
		var appendDirty = opt.DirtySemver && output.Dirty
		if output.Distance > 0 || opt.LongSemver || appendDirty {
			build = distanceS + "." + output.Hash
			if appendDirty {
				rg := regexp.MustCompile("[^a-z0-9.]")
				build += "." + rg.ReplaceAllString(opt.DirtyMark, "")
			}
		}
		if output.Semver != nil {
			output.SemverString = output.Semver.String()
			if build != "" {
				output.SemverString += "+" + build
			}
		}
	}

	if !opt.Long && output.Tag != "" && output.Distance == 0 {
		output.Suffix = ""
		output.Raw = output.Tag
	} else {
		output.Suffix = strings.Join(suffixTokens, "-")
	}

	if output.Dirty {
		output.Suffix += opt.DirtyMark
	}

	return output, nil
}

func slicePop[T any](s []T) ([]T, T) {
	i := len(s) - 1
	elem := s[i]
	s = append(s[:i], s[i+1:]...)
	return s, elem
}
