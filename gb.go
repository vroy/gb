package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	git "github.com/libgit2/git2go"
)

type ColorType string

const (
	Red    ColorType = "\x1b[0;31m"
	Yellow           = "\x1b[0;33m"
	Green            = "\x1b[0;32m"

	BaseBranch string = "master"
)

func exit(msg string, args ...string) {
	fmt.Printf(msg, args)
	os.Exit(1)
}

func NewRepo() *git.Repository {
	repo, err := git.OpenRepository(".")
	if err != nil {
		// @todo improve message
		exit("Could not open repository at '.'\n")
	}
	return repo
}

func NewBranchIterator(repo *git.Repository) *git.BranchIterator {
	i, err := repo.NewBranchIterator(git.BranchLocal)
	if err != nil {
		// @todo improve message
		exit("Can't list branches\n")
	}
	return i
}

func LookupBaseOid(repo *git.Repository) *git.Oid {
	base_branch, err := repo.LookupBranch(BaseBranch, git.BranchLocal)
	if err != nil {
		exit("Error looking up %s\n", BaseBranch)
	}

	return base_branch.Target()
}

type Comparison struct {
	Repo    *git.Repository
	BaseOid *git.Oid
	Branch  *git.Branch
	Oid     *git.Oid

	ahead  int
	behind int
}

func NewComparison(repo *git.Repository, base_oid *git.Oid, branch *git.Branch) *Comparison {
	c := new(Comparison)

	c.Repo = repo
	c.BaseOid = base_oid

	c.Branch = branch
	c.Oid = branch.Target()

	c.ahead = -1
	c.behind = -1

	return c
}

func (c *Comparison) Name() string {
	name, err := c.Branch.Name()
	if err != nil {
		exit("Can't get branch name\n")
	}
	return name
}

func (c *Comparison) IsHead() bool {
	head, err := c.Branch.IsHead()
	if err != nil {
		exit("Can't get IsHead\n")
	}
	return head
}

func (c *Comparison) IsMerged() bool {
	if c.Oid.String() == c.BaseOid.String() {
		return true
	} else {
		merged, err := c.Repo.DescendantOf(c.BaseOid, c.Oid)
		if err != nil {
			exit("Could not get descendant of '%s' and '%s'.\n", c.BaseOid.String(), c.Oid.String())
		}
		return merged
	}
}

func (c *Comparison) Commit() *git.Commit {
	commit, err := c.Repo.LookupCommit(c.Oid)
	if err != nil {
		exit("Could not lookup commit '%s'.\n", c.Oid.String())
	}
	return commit
}

// @todo red for old commits
func (c *Comparison) Color() ColorType {
	if c.IsHead() {
		return Green
	} else {
		return Yellow
	}
}

func (c *Comparison) When() time.Time {
	sig := c.Commit().Committer()
	return sig.When
}

func (c *Comparison) FormattedWhen() string {
	return c.When().Format("2006-01-02 15:04")
}

func (c *Comparison) Ahead() int {
	c.ComputeAheadBehind()
	return c.ahead
}

func (c *Comparison) Behind() int {
	c.ComputeAheadBehind()
	return c.behind
}

func (c *Comparison) ComputeAheadBehind() {
	if c.ahead > -1 && c.behind > -1 {
		return
	}

	var err error
	c.ahead, c.behind, err = c.Repo.AheadBehind(c.Oid, c.BaseOid)
	if err != nil {
		exit("Error getting ahead/behind\n", c.BaseOid.String())
	}
}

type Comparisons []*Comparison

type ComparisonsByWhen Comparisons

func (a ComparisonsByWhen) Len() int {
	return len(a)
}

func (a ComparisonsByWhen) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ComparisonsByWhen) Less(i, j int) bool {
	return a[i].When().Unix() < a[j].When().Unix()
}

type Options struct {
	Ahead    int
	Behind   int
	Merged   bool
	NoMerged bool
}

func NewOptions() *Options {
	o := new(Options)

	flag.IntVar(&o.Ahead, "ahead", -1, "help message for ahead")
	flag.IntVar(&o.Behind, "behind", -1, "help message for behind")
	flag.BoolVar(&o.Merged, "merged", false, "help message for merged")
	flag.BoolVar(&o.NoMerged, "no-merged", false, "help message for no-merged")

	flag.Parse()

	return o
}

func main() {
	opts := NewOptions()

	fmt.Printf("%s\n", opts)

	repo := NewRepo()
	branch_iterator := NewBranchIterator(repo)
	base_oid := LookupBaseOid(repo)

	comparisons := make(Comparisons, 0)

	// type BranchIteratorFunc func(*Branch, BranchType) error
	branch_iterator.ForEach(func(branch *git.Branch, btype git.BranchType) error {
		comp := NewComparison(repo, base_oid, branch)
		comparisons = append(comparisons, comp)
		return nil
	})

	sort.Sort(ComparisonsByWhen(comparisons))

	for _, comp := range comparisons {
		merged_string := ""
		if comp.IsMerged() {
			merged_string = "(merged)"
		}

		if opts.Ahead != -1 && opts.Ahead != comp.Ahead() {
			continue
		}

		if opts.Behind != -1 && opts.Behind != comp.Behind() {
			continue
		}

		if opts.Merged && !comp.IsMerged() {
			continue
		}

		if opts.NoMerged && comp.IsMerged() {
			continue
		}

		// continue
		fmt.Printf(
			"%s%s | %-30s           | behind: %4d | ahead: %4d %s\n",
			comp.Color(),
			comp.FormattedWhen(),
			comp.Name(),
			comp.Behind(),
			comp.Ahead(),
			merged_string)
	}

	// @todo store all comparisons in a list that can be sorted before printing.
	// @todo filter them things
}
