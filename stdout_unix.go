// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package suss

import (
	"io/ioutil"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// redirectOutput redirects stdout and stderr into
// a file so that printing during a test does not
// spew to the terminal. This allows us to only
// print the output from the final shrinked test
func redirectOutput() (*os.File, func(), error) {
	// grab the FDs so that we can reestablish
	// stdout/stderr
	// note that since we only close FDs that already
	// have copies elsewhere, it's not possible for them
	// to return errors
	// tempfile should be Close-on-exec, so we don't
	// have to do anything special to it
	tmpfile, err := ioutil.TempFile("", "suss")
	if err != nil {
		return nil, nil, err
	}
	stdout, stderr, err := getCopies()
	if err != nil {
		return nil, nil, err
	}

	// these dups will be not be close-on-exec,
	// which is what we want. If a test executes,
	// we should grab its stdout/stderr into the tmpfile
	err = unix.Dup2(int(tmpfile.Fd()), int(os.Stdout.Fd()))
	if err != nil {
		unix.Close(stdout)
		unix.Close(stderr)
		return nil, nil, err
	}
	err = unix.Dup2(int(tmpfile.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		unix.Close(stderr)
		// dilemma, if we manage to dup stdout, but not
		// stderr, then we should reestablish stdout
		// however that can fail. Make a best attempt effort at reestablishing it
		_ = unix.Dup2(stdout, int(os.Stdout.Fd()))
		unix.Close(stdout)
		return nil, nil, err
	}
	// stdout and stdin are now pointed to our file and
	// the fds used for reestablishing them are close-on-exec
	return tmpfile, dupAndClose(stdout, stderr), nil
}

func dupAndClose(stdout, stderr int) func() {
	f := func() {
		err := unix.Dup2(stdout, int(os.Stdout.Fd()))
		if err != nil {
			panic(err)
		}
		err = unix.Dup2(stderr, int(os.Stderr.Fd()))
		if err != nil {
			panic(err)
		}
		unix.Close(stdout)
		unix.Close(stderr)
	}
	return f
}

// Unix is hard. The spare duplicated file descriptors
// should be close-on-exec in case a test executes
// a binary.
func getCopies() (stdout int, stderr int, err error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	stdout, err = unix.Dup(int(os.Stdout.Fd()))
	if err != nil {
		return 0, 0, err
	}
	stderr, err = unix.Dup(int(os.Stdout.Fd()))
	if err != nil {
		unix.Close(stdout)
		return 0, 0, err
	}
	// these functions do not return an error.
	// not handling it is intentional
	unix.CloseOnExec(stdout)
	unix.CloseOnExec(stderr)
	return stdout, stderr, nil
}
