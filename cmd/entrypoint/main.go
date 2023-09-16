/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/platforms"
	"github.com/tektoncd/pipeline/cmd/entrypoint/subcommands"
	featureFlags "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/credentials"
	"github.com/tektoncd/pipeline/pkg/credentials/dockercreds"
	"github.com/tektoncd/pipeline/pkg/credentials/gitcreds"
	"github.com/tektoncd/pipeline/pkg/entrypoint"
	"github.com/tektoncd/pipeline/pkg/spire"
	"github.com/tektoncd/pipeline/pkg/spire/config"
	"github.com/tektoncd/pipeline/pkg/termination"
)

var (
	ep                  = flag.String("entrypoint", "", "Original specified entrypoint to execute")
	waitFiles           = flag.String("wait_file", "", "Comma-separated list of paths to wait for")
	waitFileContent     = flag.Bool("wait_file_content", false, "If specified, expect wait_file to have content")
	postFile            = flag.String("post_file", "", "If specified, file to write upon completion")
	terminationPath     = flag.String("termination_path", "/tekton/termination", "If specified, file to write upon termination")
	results             = flag.String("results", "", "If specified, list of file names that might contain task results")
	timeout             = flag.Duration("timeout", time.Duration(0), "If specified, sets timeout for step")
	stdoutPath          = flag.String("stdout_path", "", "If specified, file to copy stdout to")
	stderrPath          = flag.String("stderr_path", "", "If specified, file to copy stderr to")
	breakpointOnFailure = flag.Bool("breakpoint_on_failure", false, "If specified, expect steps to not skip on failure")
	onError             = flag.String("on_error", "", "Set to \"continue\" to ignore an error and continue when a container terminates with a non-zero exit code."+
		" Set to \"stopAndFail\" to declare a failure with a step error and stop executing the rest of the steps.")
	stepMetadataDir        = flag.String("step_metadata_dir", "", "If specified, create directory to store the step metadata e.g. /tekton/steps/<step-name>/")
	enableSpire            = flag.Bool("enable_spire", false, "If specified by configmap, this enables spire signing and verification")
	socketPath             = flag.String("spire_socket_path", "unix:///spiffe-workload-api/spire-agent.sock", "Experimental: The SPIRE agent socket for SPIFFE workload API.")
	resultExtractionMethod = flag.String("result_from", featureFlags.ResultExtractionMethodTerminationMessage, "The method using which to extract results from tasks. Default is using the termination message.")
	stepName               = flag.String("step_name", "", "Name of the step")
	stepResults            = flag.String("step_results", "", "step results if specified")
)

const (
	defaultWaitPollingInterval = time.Second
	breakpointExitSuffix       = ".breakpointexit"
)

func checkForBreakpointOnFailure(e entrypoint.Entrypointer, breakpointExitPostFile string) {
	if e.BreakpointOnFailure {
		if waitErr := e.Waiter.Wait(breakpointExitPostFile, false, false); waitErr != nil {
			log.Println("error occurred while waiting for " + breakpointExitPostFile + " : " + waitErr.Error())
		}
		// get exitcode from .breakpointexit
		exitCode, readErr := e.BreakpointExitCode(breakpointExitPostFile)
		// if readErr exists, the exitcode with default to 0 as we would like
		// to encourage to continue running the next steps in the taskRun
		if readErr != nil {
			log.Println("error occurred while reading breakpoint exit code : " + readErr.Error())
		}
		os.Exit(exitCode)
	}
}

func main() {
	// Add credential flags originally introduced with our legacy credentials helper
	// image (creds-init).
	gitcreds.AddFlags(flag.CommandLine)
	dockercreds.AddFlags(flag.CommandLine)

	// Split args with `--` for the entrypoint and what it should execute
	args, commandArgs := extractArgs(os.Args[1:])

	// We are using the global variable flag.CommandLine here to be able
	// to define what args it should parse.
	// flag.Parse() does flag.CommandLine.Parse(os.Args[1:])
	if err := flag.CommandLine.Parse(args); err != nil {
		os.Exit(1)
	}
	if err := subcommands.Process(flag.CommandLine.Args()); err != nil {
		log.Println(err.Error())
		var ok subcommands.OK
		if errors.As(err, &ok) {
			return
		}
		os.Exit(1)
	}

	// Copy credentials we're expecting from the legacy credentials helper (creds-init)
	// from secret volume mounts to /tekton/creds. This is done to support the expansion
	// of a variable, $(credentials.path), that resolves to a single place with all the
	// stored credentials.
	builders := []credentials.Builder{dockercreds.NewBuilder(), gitcreds.NewBuilder()}
	for _, c := range builders {
		if err := c.Write(pipeline.CredsDir); err != nil {
			log.Printf("Error initializing credentials: %s", err)
		}
	}

	var cmd []string
	if *ep != "" {
		cmd = []string{*ep}
	} else {
		env := os.Getenv("TEKTON_PLATFORM_COMMANDS")
		var cmds map[string][]string
		if err := json.Unmarshal([]byte(env), &cmds); err != nil {
			log.Fatal(err)
		}
		// NB: This value contains OS/architecture and maybe variant.
		// It doesn't include osversion, which is necessary to
		// disambiguate two images both for e.g., Windows, that only
		// differ by osversion.
		plat := platforms.DefaultString()
		var err error
		cmd, err = selectCommandForPlatform(cmds, plat)
		if err != nil {
			log.Fatal(err)
		}
	}

	var spireWorkloadAPI spire.EntrypointerAPIClient
	if enableSpire != nil && *enableSpire && socketPath != nil && *socketPath != "" {
		spireConfig := config.SpireConfig{
			SocketPath: *socketPath,
		}
		spireWorkloadAPI = spire.NewEntrypointerAPIClient(&spireConfig)
	}

	stepRes := map[string]string{}
	json.Unmarshal([]byte(*stepResults), &stepRes)
	e := entrypoint.Entrypointer{
		Command:         append(cmd, commandArgs...),
		WaitFiles:       strings.Split(*waitFiles, ","),
		WaitFileContent: *waitFileContent,
		PostFile:        *postFile,
		TerminationPath: *terminationPath,
		Waiter:          &realWaiter{waitPollingInterval: defaultWaitPollingInterval, breakpointOnFailure: *breakpointOnFailure},
		Runner: &realRunner{
			stdoutPath: *stdoutPath,
			stderrPath: *stderrPath,
		},
		PostWriter:             &realPostWriter{},
		Results:                strings.Split(*results, ","),
		StepResults:            stepRes,
		Timeout:                timeout,
		BreakpointOnFailure:    *breakpointOnFailure,
		OnError:                *onError,
		StepMetadataDir:        *stepMetadataDir,
		SpireWorkloadAPI:       spireWorkloadAPI,
		StepName:               *stepName,
		ResultExtractionMethod: *resultExtractionMethod,
	}

	// Copy any creds injected by the controller into the $HOME directory of the current
	// user so that they're discoverable by git / ssh.
	if err := credentials.CopyCredsToHome(credentials.CredsInitCredentials); err != nil {
		log.Printf("non-fatal error copying credentials: %q", err)
	}

	if err := e.Go(); err != nil {
		breakpointExitPostFile := e.PostFile + breakpointExitSuffix
		switch t := err.(type) { //nolint:errorlint // checking for multiple types with errors.As is ugly.
		case skipError:
			log.Print("Skipping step because a previous step failed")
			os.Exit(1)
		case termination.MessageLengthError:
			log.Print(err.Error())
			os.Exit(1)
		case *exec.ExitError:
			// Copied from https://stackoverflow.com/questions/10385551/get-exit-code-go
			// This works on both Unix and Windows. Although
			// package syscall is generally platform dependent,
			// WaitStatus is defined for both Unix and Windows and
			// in both cases has an ExitStatus() method with the
			// same signature.
			if status, ok := t.Sys().(syscall.WaitStatus); ok {
				checkForBreakpointOnFailure(e, breakpointExitPostFile)
				// ignore a step error i.e. do not exit if a container terminates with a non-zero exit code when onError is set to "continue"
				if e.OnError != entrypoint.ContinueOnError {
					os.Exit(status.ExitStatus())
				}
			}
			// log and exit only if a step error must cause run failure
			if e.OnError != entrypoint.ContinueOnError {
				log.Fatalf("Error executing command (ExitError): %v", err)
			}
		default:
			checkForBreakpointOnFailure(e, breakpointExitPostFile)
			log.Fatalf("Error executing command: %v", err)
		}
	}
}

func selectCommandForPlatform(cmds map[string][]string, plat string) ([]string, error) {
	cmd, found := cmds[plat]
	if found {
		return cmd, nil
	}

	// If the command wasn't found, check if there's a
	// command defined for the same platform without a CPU
	// variant specified.
	platWithoutVariant := plat[:strings.LastIndex(plat, "/")]
	cmd, found = cmds[platWithoutVariant]
	if found {
		return cmd, nil
	}
	return nil, fmt.Errorf("could not find command for platform %q", plat)
}
