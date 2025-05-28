package bgjob

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type BackgroundJob struct {
	Id         int           // job id
	Name       string        // job name
	Command    []string      // the command to execute
	Created    time.Time     // time at which the job was created
	Started    time.Time     // time at which the job started
	Duration   time.Duration // how long the job took to run
	Stdout     string        // stdout, if any, from the job
	Stderr     string        // stderr, if any, from the job
	ExitStatus int           // the exit status of the job itself
	Error      error         // the error returned from the exec.Cmd.Run()
	CmdError   error         // the error found in exec.Cmd.Err
}

func New(id int, prefix string, command []string) (bj *BackgroundJob) {
	bj = new(BackgroundJob)
	bj.Init(id, prefix, command)
	return
}

func (bj *BackgroundJob) Init(id int, prefix string, command []string) {
	bj.Id = id
	bj.setName(prefix)
	bj.Command = command
}

func (bj *BackgroundJob) MarkReady() {
	bj.Created = time.Now()
}

func (bj *BackgroundJob) MarkStarted() {
	bj.Started = time.Now()
}

func (bj *BackgroundJob) MarkFinished() {
	bj.Duration = time.Now().Sub(bj.Started)
}

func (bj *BackgroundJob) setName(prefix string) {
	bj.Name = fmt.Sprintf("%s_%06d", prefix, bj.Id)
}

func (bj *BackgroundJob) printBanner(subFmt string, subArgs ...any) {
	args := append([]any{bj.Name}, subArgs...)

	fmt.Printf("[%s job "+subFmt+"]\n", args...)
}

func (bj *BackgroundJob) Run() (err error) {
	// create a Command struct to manage running the command, using
	// Buffers for the stdout and stderr
	cmd := exec.Command(bj.Command[0], bj.Command[1:]...)
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)

	// run the specified command and wait for it to complete,
	// marking it as finished once done.
	bj.Error = cmd.Run()
	bj.MarkFinished()

	// record the stdout and stderr
	bj.Stdout = cmd.Stdout.(*bytes.Buffer).String()
	bj.Stderr = cmd.Stderr.(*bytes.Buffer).String()

	// extract the exit status if the job command ran but failed
	if bj.Error != nil {
		// extract the exit status if the the command ran but failed.
		if exitError, ok := bj.Error.(*exec.ExitError); ok {
			bj.ExitStatus = exitError.ExitCode()
		} else {
			// otherwise there was an error with the command itself, so
			// set the exit status to -1
			bj.ExitStatus = -1
		}
		if cmd.Err != nil {
			bj.CmdError = cmd.Err
		}
	}

	err = bj.Error

	return
}

// Print BackgroundJob results
func (bj *BackgroundJob) Print(detailed bool) {
	// calculate time deltas
	jobStartDelta := bj.Started.Sub(bj.Created)

	if bj.ExitStatus < 0 {
		bj.printBanner("command invalid")
		fmt.Printf("Command: %s\n", strings.Join(bj.Command, " "))
		if bj.Error != nil {
			fmt.Printf("Error: %s\n", bj.Error.Error())
		}
		if bj.CmdError != nil {
			fmt.Printf("Cmd.Err: %s\n", bj.CmdError.Error())
		}

		return
	}

	if detailed || bj.ExitStatus > 0 {
		if bj.Stdout != "" {
			bj.printBanner("stdout")
			fmt.Printf("%s", bj.Stdout)
			// append a newline if the string doesn't end with one
			if !strings.HasSuffix(bj.Stdout, "\n") {
				fmt.Println()
			}
		} else {
			bj.printBanner("stdout empty")
		}

		if bj.Stderr != "" {
			bj.printBanner("stderr")
			fmt.Printf("%s", bj.Stderr)
			// append a newline if the string doesn't end with one
			if !strings.HasSuffix(bj.Stderr, "\n") {
				fmt.Println()
			}
		} else {
			bj.printBanner("stderr empty")
		}
	}

	bj.printBanner(
		"exit status: %3d, times: %+13.6fs (%13.6fs)",
		bj.ExitStatus,
		jobStartDelta.Seconds(),
		bj.Duration.Seconds(),
	)
}
