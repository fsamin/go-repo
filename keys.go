package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// sshKey is a type for a ssh key
type sshKey struct {
	filename string
	content  []byte
}

// pgpKey is a type for a pgp key
type pgpKey struct {
	name    string
	public  string
	private string
	id      string
}

func (r Repo) setupSSHKey() ([]string, error) {
	if r.sshKey == nil {
		return nil, fmt.Errorf("no ssh keys to setup")
	}

	sshPath := exec.Command("ssh").Path
	escapedKeyFile := shellEscape(r.sshKey.filename)

	gitSSHCmd := sshPath + " -i " + escapedKeyFile
	gitSSHCmd += " -o IdentitiesOnly=yes"

	// Use accept-new by default: accepts new host keys but rejects changed ones.
	// Only disable StrictHostKeyChecking if explicitly requested via WithStrictHostKeyCheckingDisabled.
	if r.disableStrictHostKeyChk {
		gitSSHCmd += " -o StrictHostKeyChecking=no"
	} else {
		gitSSHCmd += " -o StrictHostKeyChecking=accept-new"
	}

	keyDir := filepath.Dir(r.sshKey.filename)

	var wrapper, wrapperPath string
	if runtime.GOOS == "windows" {
		gitSSHCmd += ` %*`
		wrapper = `@echo off
` + gitSSHCmd
		wrapperPath = filepath.Join(keyDir, "gitwrapper.bat")
	} else {
		gitSSHCmd += ` "$@"`
		wrapper = `#!/bin/sh
` + gitSSHCmd
		wrapperPath = filepath.Join(keyDir, "gitwrapper")
	}

	if err := os.WriteFile(wrapperPath, []byte(wrapper), os.FileMode(0700)); err != nil {
		return nil, err
	}

	return []string{"GIT_SSH=" + wrapperPath, "PKEY=" + r.sshKey.filename}, nil
}

func (r Repo) installGPGKey() error {
	return nil
}
