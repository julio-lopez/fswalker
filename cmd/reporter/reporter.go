// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Reporter is a CLI tool to process file system report files generated by Walker.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/google/fswalker"
)

var (
	configFile   = flag.String("config-file", "", "required report config file to use")
	walkPath     = flag.String("walk-path", "", "path to search for Walks")
	reviewFile   = flag.String("review-file", "", "path to the file containing a list of last-known-good states - this needs to be writeable")
	hostname     = flag.String("hostname", "", "host to review the differences for")
	beforeFile   = flag.String("before-file", "", "path to the file to compare against (last known good typically)")
	afterFile    = flag.String("after-file", "", "path to the file to compare with the before state")
	paginate     = flag.Bool("paginate", false, "pipe output into $PAGER in order to paginate and make reviews easier")
	verbose      = flag.Bool("verbose", false, "print additional output for each file which changed")
	updateReview = flag.Bool("update-review", false, "ask to update the \"last known good\" review")
)

func askUpdateReviews() bool {
	fmt.Print("Do you want to update the \"last known good\" to this [y/N]: ")
	var input string
	fmt.Scanln(&input)
	return strings.ToLower(strings.TrimSpace(input)) == "y"
}

func walksByLatest(ctx context.Context, r *fswalker.Reporter, hostname, reviewFile, walkPath string) (*fswalker.WalkFile, *fswalker.WalkFile, error) {
	before, err := r.ReadLastGoodWalk(ctx, hostname, reviewFile)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load last good walk for %s: %v", hostname, err)
	}
	after, err := r.ReadLatestWalk(ctx, hostname, walkPath)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load latest walk for %s: %v", hostname, err)
	}
	return before, after, nil
}

func walksByFiles(ctx context.Context, r *fswalker.Reporter, beforeFile, afterFile string) (*fswalker.WalkFile, *fswalker.WalkFile, error) {
	after, err := r.ReadWalk(ctx, afterFile)
	if err != nil {
		return nil, nil, fmt.Errorf("file cannot be read: %s", afterFile)
	}
	var before *fswalker.WalkFile
	if beforeFile != "" {
		before, err = r.ReadWalk(ctx, beforeFile)
		if err != nil {
			return nil, nil, fmt.Errorf("file cannot be read: %s", beforeFile)
		}
	}
	return before, after, nil
}

func main() {
	ctx := context.Background()
	flag.Parse()

	// Loading configs and walks.
	if *configFile == "" {
		log.Fatal("config-file needs to be specified")
	}
	rptr, err := fswalker.ReporterFromConfigFile(ctx, *configFile, *verbose)
	if err != nil {
		log.Fatal(err)
	}

	var before, after *fswalker.WalkFile
	var errWalks error
	if *hostname != "" && *reviewFile != "" && *walkPath != "" {
		if *afterFile != "" || *beforeFile != "" {
			log.Fatalf("[hostname review-file walk-path] and [[before-file] after-file] are mutually exclusive")
		}
		before, after, errWalks = walksByLatest(ctx, rptr, *hostname, *reviewFile, *walkPath)
	} else if *afterFile != "" {
		before, after, errWalks = walksByFiles(ctx, rptr, *beforeFile, *afterFile)
	} else {
		log.Fatalf("either [hostname review-file walk-path] OR [[before-file] after-file] need to be specified")
	}
	if errWalks != nil {
		log.Fatal(errWalks)
	}

	var report *fswalker.Report
	var errReport error
	if before == nil {
		report, errReport = rptr.Compare(nil, after.Walk)
	} else {
		report, errReport = rptr.Compare(before.Walk, after.Walk)
	}
	if errReport != nil {
		log.Fatal(errReport)
	}

	// Processing and output.
	// Note that we do some trickery here to allow pagination via $PAGER if requested.
	out := io.WriteCloser(os.Stdout)
	var cmd *exec.Cmd
	if *paginate {
		pager := os.Getenv("PAGER")
		if pager == "" {
			pager = "/usr/bin/less"
		}
		// Set up pager piped with the program's stdio.
		// Its stdin is closed later in this func, after all reports have been piped.
		cmd = exec.Command(pager)
		cmd.Stdout = os.Stdout
		pipein, err := cmd.StdinPipe()
		if err != nil {
			log.Fatal(err)
		}
		out = pipein
		if err := cmd.Start(); err != nil {
			log.Fatalf("unable to start %q: %v", pager, err)
		}
	}

	if before == nil {
		fmt.Fprintln(out, "No before walk found. Using after walk only.")
	}
	rptr.PrintReportSummary(out, report)
	if err := rptr.PrintRuleSummary(out, report); err != nil {
		log.Fatal(err)
	}
	rptr.PrintDiffSummary(out, report)

	fmt.Fprintln(out, "Metrics:")
	for _, k := range report.Counter.Metrics() {
		v, _ := report.Counter.Get(k)
		fmt.Fprintf(out, "[%-30s] = %6d\n", k, v)
	}

	if *paginate {
		if err := out.Close(); err != nil {
			log.Println("closing output:", err)
		}
		if err := cmd.Wait(); err != nil {
			log.Println("waiting for command:", err)
		}
	}

	// Update reviews file if desired.
	if *updateReview && askUpdateReviews() {
		if err := rptr.UpdateReviewProto(ctx, after, *reviewFile); err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Println("not updating reviews file")
	}
}
