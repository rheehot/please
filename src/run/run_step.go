// Code for running targets directly through Please.

package run

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sync/errgroup"
	"gopkg.in/op/go-logging.v1"

	"build"
	"core"
	"output"
)

var log = logging.MustGetLogger("run")

// Run implements the running part of 'plz run'.
func Run(graph *core.BuildGraph, label core.BuildLabel, args []string) {
	run(graph, label, args, false, false)
}

// Parallel runs a series of targets in parallel.
// Returns true if all were successful.
func Parallel(graph *core.BuildGraph, labels []core.BuildLabel, args []string, numTasks int, quiet bool) int {
	pool := NewGoroutinePool(numTasks)
	var g errgroup.Group
	for _, label := range labels {
		label := label // capture locally
		g.Go(func() (err error) {
			var wg sync.WaitGroup
			wg.Add(1)
			pool.Submit(func() {
				if e := run(graph, label, args, true, quiet); e != nil {
					err = e
				}
				wg.Done()
			})
			wg.Wait()
			return
		})
	}
	if err := g.Wait(); err != nil {
		log.Error("Command failed: %s", err)
		return err.(*exitError).code
	}
	return 0
}

// Sequential runs a series of targets sequentially.
// Returns true if all were successful.
func Sequential(graph *core.BuildGraph, labels []core.BuildLabel, args []string, quiet bool) int {
	for _, label := range labels {
		log.Notice("Running %s", label)
		if err := run(graph, label, args, true, quiet); err != nil {
			log.Error("%s", err)
			return err.code
		}
	}
	return 0
}

// run implements the internal logic about running a target.
// If fork is true then we fork to run the target and return any error from the subprocesses.
// If it's false this function never returns (because we either win or die; it's like
// Game of Thrones except rather less glamorous).
func run(graph *core.BuildGraph, label core.BuildLabel, args []string, fork, quiet bool) *exitError {
	target := graph.TargetOrDie(label)
	if !target.IsBinary {
		log.Fatalf("Target %s cannot be run; it's not marked as binary", label)
	}
	// ReplaceSequences always quotes stuff in case it contains spaces or special characters,
	// that works fine if we interpret it as a shell but not to pass it as an argument here.
	arg0 := strings.Trim(build.ReplaceSequences(target, fmt.Sprintf("$(out_exe %s)", target.Label)), "\"")
	// Handle targets where $(exe ...) returns something nontrivial
	splitCmd := strings.Split(arg0, " ")
	if !strings.Contains(splitCmd[0], "/") {
		// Probably it's a java -jar, we need an absolute path to it.
		cmd, err := exec.LookPath(splitCmd[0])
		if err != nil {
			log.Fatalf("Can't find binary %s", splitCmd[0])
		}
		splitCmd[0] = cmd
	}
	args = append(splitCmd, args...)
	log.Info("Running target %s...", strings.Join(args, " "))
	output.SetWindowTitle("plz run: " + strings.Join(args, " "))
	if !fork {
		// Plain 'plz run'. One way or another we never return from the following line.
		must(syscall.Exec(splitCmd[0], args, os.Environ()), args)
	}
	// Run as a normal subcommand.
	// Note that we don't connect stdin. It doesn't make sense for multiple processes.
	cmd := core.ExecCommand(splitCmd[0], args[1:]...) // args here don't include argv[0]
	if !quiet {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		must(cmd.Start(), args)
		err := cmd.Wait()
		return toExitError(err, cmd, nil)
	}
	out, err := cmd.CombinedOutput()
	return toExitError(err, cmd, out)
}

// must dies if the given error is non-nil.
func must(err error, cmd []string) {
	if err != nil {
		log.Fatalf("Error running command %s: %s", strings.Join(cmd, " "), err)
	}
}

// toExitError attempts to extract the exit code from an error.
func toExitError(err error, cmd *exec.Cmd, out []byte) *exitError {
	exitCode := 1
	if err == nil {
		return nil
	} else if exitError, ok := err.(*exec.ExitError); ok {
		// This is a little hairy; there isn't a good way of getting the exit code,
		// but this should be reasonably portable (at least to the platforms we care about).
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}
	return &exitError{
		msg:  fmt.Sprintf("Error running command %s: %s\n%s", strings.Join(cmd.Args, " "), err, string(out)),
		code: exitCode,
	}
}

type exitError struct {
	msg  string
	code int
}

func (e *exitError) Error() string {
	return e.msg
}
