package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const defaultBranch = "main"

var errWrongUsage = errors.New("wrong usage")

func main() {
	if err := run(); err != nil {
		if errors.Is(errWrongUsage, err) {
			fmt.Fprintln(os.Stderr, "Usage: go run main.go [branchname]")
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func run() error {
	var branch string
	filters := []string{
		".git/",
	}

	flag.StringVar(&branch, "branch", defaultBranch, "git branch to use")
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

	files := make(map[int][]string)
	err = filepath.WalkDir(relpath, makeWalkDir(branch, files, filters))
	if err != nil {
		return fmt.Errorf("walk directory %s: %w", basepath, err)
	}

	if len(files) == 0 {
		fmt.Println("No files found")
		return nil
	}

	counts := make([]int, 0, len(files))
	for c := range files {
		counts = append(counts, c)
	}

	sort.Ints(counts)

	highestNumOfCommits := counts[len(counts)-1]
	next := files[highestNumOfCommits]

	fmt.Println()
	fmt.Println("Next to refactor:")
	for _, p := range next {
		fmt.Printf("%d\t%s\n", highestNumOfCommits, p)
	}

	return nil
}

func getPath() (string, error) {
	return os.Getwd()
}

func makeWalkDir(branch string, files map[int][]string, filters []string) fs.WalkDirFunc {
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

		files[count] = append(files[count], path)

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
