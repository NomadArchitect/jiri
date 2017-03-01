package git

import (
	"fmt"
	git2go "github.com/libgit2/git2go"
)

type Git struct {
	rootDir string
}

func New(path string) *Git {
	return &Git{
		rootDir: path,
	}
}

func (g *Git) CurrentRevision() (string, error) {
	repo, err := git2go.OpenRepository(g.rootDir)
	if err != nil {
		return "", err
	}
	defer repo.Free()
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	defer head.Free()
	return head.Target().String(), nil
}

// Fetch fetches refs and tags from the given remote.
func (g *Git) Fetch(remote string, opts ...FetchOpt) error {
	return g.FetchRefspec(remote, "", opts...)
}

// FetchRefspec fetches refs and tags from the given remote for a particular refspec.
func (g *Git) FetchRefspec(remoteName, refspec string, opts ...FetchOpt) error {
	repo, err := git2go.OpenRepository(g.rootDir)
	if err != nil {
		return err
	}
	defer repo.Free()
	if remoteName == "" {
		return fmt.Errorf("No remote passed")
	}
	remote, err := repo.Remotes.Lookup(remoteName)
	if err != nil {
		return err
	}
	defer remote.Free()
	fetchOptions := &git2go.FetchOptions{}
	tags := false
	prune := false
	for _, opt := range opts {
		switch typedOpt := opt.(type) {
		case TagsOpt:
			tags = bool(typedOpt)
		case PruneOpt:
			prune = bool(typedOpt)
		}
	}
	refspecList := []string{}
	if refspec != "" {
		refspecList = []string{refspec}
	}
	if prune {
		fetchOptions.Prune = git2go.FetchPruneOn
	}
	if tags {
		fetchOptions.DownloadTags = git2go.DownloadTagsAll
	}
	return remote.Fetch(refspecList, fetchOptions, "")
}

func (g *Git) SetRemoteUrl(remote, url string) error {
	repo, err := git2go.OpenRepository(g.rootDir)
	if err != nil {
		return err
	}
	defer repo.Free()
	return repo.Remotes.SetUrl(remote, url)
}

type TrackingBranch string
type Revision string
type BranchName string
type IsCurrent bool
type Name string
type BranchType string

const (
	RemoteType = "remote"
	LocalType  = "local"
)

func (g *Git) GetAllBranchesInfo() (map[string]struct {
	TrackingBranch
	Revision
	IsCurrent
	BranchType
}, error) {
	repo, err := git2go.OpenRepository(g.rootDir)
	if err != nil {
		return nil, err
	}
	defer repo.Free()
	branchIterator, err := repo.NewBranchIterator(git2go.BranchAll)
	if err != nil {
		return nil, err
	}
	m := make(map[string]struct {
		TrackingBranch
		Revision
		IsCurrent
		BranchType
	})
	err = branchIterator.ForEach(func(b *git2go.Branch, bType git2go.BranchType) error {
		isHead, err := b.IsHead()
		if err != nil {
			return err
		}
		name, err := b.Name()
		if err != nil {
			return err
		}
		trackingBranch := ""
		revision := ""
		if t := b.Target(); t != nil {
			revision = t.String()
		}
		branchType := RemoteType
		if bType == git2go.BranchLocal {
			branchType = LocalType
			if u, err := b.Upstream(); err != nil && !git2go.IsErrorCode(err, git2go.ErrNotFound) {
				return err
			} else if u != nil {
				defer u.Free()
				trackingBranch = u.Shorthand()
			}
		}
		m[name] = struct {
			TrackingBranch
			Revision
			IsCurrent
			BranchType
		}{TrackingBranch(trackingBranch), Revision(revision), IsCurrent(isHead), BranchType(branchType)}
		return nil
	})
	return m, err
}
