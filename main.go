package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBranch              = "main"
	defaultMaxNumbersToDisplay = 10
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		branch              string
		maxNumbersToDisplay int
	)
	filters := []string{
		".git/",
	}

	flag.StringVar(&branch, "branch", defaultBranch, "git branch to use")
	flag.IntVar(&maxNumbersToDisplay, "n", defaultMaxNumbersToDisplay, "max numbers to display")
	flag.Func("filter", "a filter pattern", func(s string) error {
		filters = append(filters, filepath.Clean(s))
		return nil
	})
	flag.Parse()

	basepath, err := getPath()
	if err != nil {
		return fmt.Errorf("read path: %w", err)
	}

	relpath, err := filepath.Rel(basepath, basepath)
	if err != nil {
		return fmt.Errorf("make relpath: %w", err)
	}

	start := time.Now()

	var pc projectChanges
	err = filepath.WalkDir(relpath, pc.makeWalkDir(branch, filters))
	if err != nil {
		return fmt.Errorf("walk directory %s: %w", basepath, err)
	}

	end := time.Now()

	displayChanges(pc.changes, maxNumbersToDisplay)

	fmt.Println()
	fmt.Printf("Scanned %d files in %s\n", len(pc.changes), end.Sub(start))

	return nil
}

func getPath() (string, error) {
	return os.Getwd()
}

type fileChanges struct {
	path  string
	count int
}

type projectChanges struct {
	changes []fileChanges
}

func (pc *projectChanges) makeWalkDir(branch string, filters []string) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("Skipping %s because of: %s", path, err.Error())
			return filepath.SkipDir
		}

		filtered, err := filterPath(path, filters)
		if err != nil {
			return fmt.Errorf("filter path %s: %w", path, err)
		}
		if filtered {
			return filepath.SkipDir
		}

		if d.IsDir() {
			fmt.Println("Entering " + path)
			return nil
		}

		count, err := countCommits(path, branch)
		if err != nil {
			return fmt.Errorf("count commits %s: %w", path, err)
		}

		pc.changes = append(pc.changes, fileChanges{
			path:  path,
			count: count,
		})

		return nil
	}
}

func filterPath(p string, filters []string) (bool, error) {
	for _, f := range filters {
		f = filepath.Clean(f)
		match, err := filepath.Match(f, p)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}

	return false, nil
}

func countCommits(path, branch string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", branch, "--", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
		return 0, fmt.Errorf("run git rev-list: %w", err)
	}

	trimmed := strings.Trim(string(out), " \n")

	count, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parse count: %w", err)
	}

	return count, nil
}

func displayChanges(changes []fileChanges, n int) {
	if len(changes) == 0 {
		fmt.Println("No file changes found")
		return
	}

	if n > len(changes) {
		n = len(changes)
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].count > changes[j].count
	})

	fmt.Println()
	fmt.Println("Next to refactor:")

	fmt.Println()
	fmt.Printf("commits\tfile\n")
	fmt.Printf("-------\t----\n")

	highest := changes[:n]
	for _, c := range highest {
		fmt.Printf("%d\t%s\n", c.count, c.path)
	}
}
