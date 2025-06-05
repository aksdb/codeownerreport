package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/hmarr/codeowners"
	"github.com/samber/lo"
)

func main() {
	ruleset, err := loadRuleset()
	if err != nil {
		slog.Error("Error loading ruleset.", "error", err)
		os.Exit(1)
	}

	repo, err := git.PlainOpen(".")
	if err != nil {
		slog.Error("Error opening repository.", "error", err)
		os.Exit(1)
	}

	currentBranch, err := repo.Head()
	if err != nil {
		slog.Error("Error getting current branch.", "error", err)
		os.Exit(1)
	}
	if !currentBranch.Name().IsBranch() {
		slog.Error("Not on a branch.")
		os.Exit(1)
	}
	slog.Info("Selected current branch.", "branch", currentBranch.Name().Short())

	mainBranch, err := repo.Branch("main")
	if errors.Is(err, git.ErrBranchNotFound) {
		mainBranch, err = repo.Branch("master")
	}
	if err != nil {
		slog.Error("Error finding main branch.", "error", err)
		os.Exit(1)
	}

	mainRef, err := repo.Reference(mainBranch.Merge, true)
	if err != nil {
		slog.Error("Error resolving main branch to reference.", "error", err)
		os.Exit(1)
	}
	mainCommit, err := repo.CommitObject(mainRef.Hash())
	if err != nil {
		slog.Error("Error resolving main branch to commit.", "error", err)
		os.Exit(1)
	}

	slog.Info("Selected reference branch.", "branch", mainBranch.Name)

	currentCommit, err := repo.CommitObject(currentBranch.Hash())
	if err != nil {
		slog.Error("Error resolving HEAD commit.", "error", err)
		os.Exit(1)
	}

	baseCommits, err := currentCommit.MergeBase(mainCommit)
	if err != nil {
		slog.Error("Error resolving merge base commit.", "error", err)
		os.Exit(1)
	}

	if len(baseCommits) < 1 {
		slog.Error("Could not find merge base.")
		os.Exit(1)
	}

	baseCommit := baseCommits[0]

	slog.Info("Identified base commit.", "commit", baseCommit)

	baseTree, err := baseCommit.Tree()
	if err != nil {
		slog.Error("Error getting base commit tree.", "error", err)
		os.Exit(1)
	}

	currentTree, err := currentCommit.Tree()
	if err != nil {
		slog.Error("Error getting current commit tree.", "error", err)
		os.Exit(1)
	}

	diff, err := baseTree.Diff(currentTree)
	if err != nil {
		slog.Error("Error determining diff between trees.", "error", err)
		os.Exit(1)
	}

	patch, err := diff.Patch()
	if err != nil {
		slog.Error("Error getting patch from diff.", "error", err)
		os.Exit(1)
	}

	fileOwners := map[string][]string{}

	for _, fp := range patch.FilePatches() {
		from, to := fp.Files()
		if from != nil {
			fileOwners[from.Path()] = nil
		}
		if to != nil {
			fileOwners[to.Path()] = nil
		}
	}

	for file := range fileOwners {
		rule, err := ruleset.Match(file)
		if err != nil {
			slog.Error("Failed to match rule for file.", "file", file, "error", err)
			continue
		}
		fileOwners[file] = lo.Map(rule.Owners, func(owner codeowners.Owner, index int) string {
			return owner.String()
		})
	}

	ownerFiles := map[string][]string{}
	for file, owners := range fileOwners {
		for _, owner := range owners {
			ownerFiles[owner] = append(ownerFiles[owner], file)
		}
	}
	for owner := range ownerFiles {
		files := lo.Uniq(ownerFiles[owner])
		fmt.Println()
		fmt.Println(owner)
		for _, file := range files {
			fmt.Printf("  %s\n", file)
		}
	}
}

func loadRuleset() (codeowners.Ruleset, error) {
	f, err := os.Open(".github/CODEOWNERS")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return codeowners.ParseFile(f)
}
