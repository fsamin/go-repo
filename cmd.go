package repo

import (
	"bytes"
	"context"
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

func (r Repo) runCmd(ctx context.Context, name string, args ...string) (stdOut string, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	buffOut := new(bytes.Buffer)
	buffErr := new(bytes.Buffer)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Dir = r.path
	cmd.Stderr = buffErr
	cmd.Stdout = buffOut

	if r.sshKey != nil {
		envs, err := r.setupSSHKey()
		if err != nil {
			return "", err
		}
		cmd.Env = append(cmd.Env, envs...)
		if r.verbose {
			r.log("Using %v\n", envs)
		}
	}

	if r.verbose {
		r.log("Running command %+v\n", cmd)
	}

	// set lang to english to be able to parse git messages
	cmd.Env = append(cmd.Env, "LANG=en_US")

	runErr := cmd.Run()

	stdOut = buffOut.String()
	stdErr := buffErr.String()

	if cmd.ProcessState == nil || !cmd.ProcessState.Success() {
		if len(stdErr) > 0 {
			return stdOut, errors.Errorf("%s (%v)", stdErr, runErr)
		}
		return stdOut, errors.Errorf("exited with error: %v", runErr)
	}

	if runErr != nil {
		return stdOut, errors.Errorf("%s (%v)", stdErr, runErr)
	}

	btes := []byte(stdOut)
	// replace CR LF \r\n (windows) with LF \n (unix)
	btes = bytes.Replace(btes, []byte{13, 10}, []byte{10}, -1)
	// replace CF \r (mac) with LF \n (unix)
	btes = bytes.Replace(btes, []byte{13}, []byte{10}, -1)

	return string(btes), nil
}
