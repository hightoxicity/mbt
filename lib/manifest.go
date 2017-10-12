package lib

import (
	"encoding/hex"
	"sort"
	"strings"

	git "github.com/libgit2/git2go"
	yaml "gopkg.in/yaml.v2"
)

type BuildCmd struct {
	Cmd  string
	Args []string `yaml:",flow"`
}

type Spec struct {
	Version    string
	Name       string
	Build      map[string]*BuildCmd
	Properties map[string]interface{}
}

type Manifest struct {
	Dir          string
	Sha          string
	Applications Applications
}

func ManifestByPr(dir, src, dst string) (*Manifest, error) {
	repo, m, err := openRepo(dir)
	if err != nil {
		return nil, err
	}

	if m != nil {
		return m, nil
	}

	srcC, err := getBranchCommit(repo, src)
	if err != nil {
		return nil, err
	}

	dstC, err := getBranchCommit(repo, dst)
	if err != err {
		return nil, err
	}

	diff, err := getDiffFromMergeBase(repo, srcC, dstC)
	if err != nil {
		return nil, err
	}

	m, err = fromBranch(repo, dir, src)
	if err != nil {
		return nil, err
	}

	return reduceToDiff(m, diff)
}

func ManifestBySha(dir, sha string) (*Manifest, error) {
	repo, m, err := openRepo(dir)
	if err != nil {
		return nil, err
	}

	if m != nil {
		return m, nil
	}

	bytes, err := hex.DecodeString(sha)
	if err != nil {
		return nil, err
	}

	oid := git.NewOidFromBytes(bytes)
	commit, err := repo.LookupCommit(oid)
	if err != nil {
		return nil, err
	}

	return fromCommit(repo, dir, commit)
}

func ManifestByBranch(dir, branch string) (*Manifest, error) {
	repo, m, err := openRepo(dir)
	if err != nil {
		return nil, err
	}

	if m != nil {
		return m, nil
	}

	return fromBranch(repo, dir, branch)
}

func ManifestByDiff(dir, from, to string) (*Manifest, error) {
	repo, m, err := openRepo(dir)
	if err != nil {
		return nil, err
	}

	if m != nil {
		return m, nil
	}

	fromOid, err := git.NewOid(from)
	if err != nil {
		return nil, err
	}

	toOid, err := git.NewOid(to)
	if err != nil {
		return nil, err
	}

	fromC, err := repo.LookupCommit(fromOid)
	if err != nil {
		return nil, err
	}

	toC, err := repo.LookupCommit(toOid)
	if err != nil {
		return nil, err
	}

	diff, err := getDiffFromMergeBase(repo, toC, fromC)
	if err != nil {
		return nil, err
	}

	m, err = fromCommit(repo, dir, toC)
	if err != nil {
		return nil, err
	}

	return reduceToDiff(m, diff)
}

func (m *Manifest) indexByName() map[string]*Application {
	return m.Applications.indexByName()
}

func (m *Manifest) indexByPath() map[string]*Application {
	return m.Applications.indexByPath()
}

func fromCommit(repo *git.Repository, dir string, commit *git.Commit) (*Manifest, error) {
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	vapps := Applications{}

	err = tree.Walk(func(path string, entry *git.TreeEntry) int {
		if entry.Name == ".mbt.yml" && entry.Type == git.ObjectBlob {
			blob, err := repo.LookupBlob(entry.Id)
			if err != nil {
				return 1
			}

			version := ""

			p := strings.TrimRight(path, "/")
			if p != "" {
				// We are not on the root, take the git sha for parent tree object.
				dirEntry, err := tree.EntryByPath(p)
				if err != nil {
					return 1
				}
				version = dirEntry.Id.String()
			} else {
				// We are on the root, take the commit sha.
				version = commit.Id().String()
			}

			a, err := newApplication(p, version, blob.Contents())
			if err != nil {
				// TODO log this or fail
				return 1
			}

			vapps = append(vapps, a)
		}
		return 0
	})

	if err != nil {
		return nil, err
	}

	sort.Sort(vapps)
	return &Manifest{dir, commit.Id().String(), vapps}, nil
}

func newApplication(dir, version string, spec []byte) (*Application, error) {
	a := &Spec{
		Properties: make(map[string]interface{}),
		Build:      make(map[string]*BuildCmd),
	}

	err := yaml.Unmarshal(spec, a)
	if err != nil {
		return nil, err
	}

	return &Application{
		Build:      a.Build,
		Name:       a.Name,
		Properties: a.Properties,
		Version:    version,
		Path:       dir,
	}, nil
}

func newEmptyManifest(dir string) *Manifest {
	return &Manifest{Applications: []*Application{}, Dir: dir, Sha: ""}
}

func fromBranch(repo *git.Repository, dir string, branch string) (*Manifest, error) {
	commit, err := getBranchCommit(repo, branch)
	if err != nil {
		return nil, err
	}

	return fromCommit(repo, dir, commit)
}

func reduceToDiff(manifest *Manifest, diff *git.Diff) (*Manifest, error) {
	q := manifest.indexByPath()
	filtered := make(map[string]*Application)
	err := diff.ForEach(func(delta git.DiffDelta, num float64) (git.DiffForEachHunkCallback, error) {
		for k := range q {
			if _, ok := filtered[k]; ok {
				continue
			}
			if strings.HasPrefix(delta.NewFile.Path, k) {
				filtered[k] = q[k]
			}
		}
		return nil, nil
	}, git.DiffDetailFiles)

	if err != nil {
		return nil, err
	}

	apps := []*Application{}
	for _, v := range filtered {
		apps = append(apps, v)
	}

	return &Manifest{
		Dir:          manifest.Dir,
		Sha:          manifest.Sha,
		Applications: apps,
	}, nil
}

func openRepo(dir string) (*git.Repository, *Manifest, error) {
	repo, err := git.OpenRepository(dir)
	if err != nil {
		return nil, nil, err
	}
	empty, err := repo.IsEmpty()
	if err != nil {
		return nil, nil, err
	}

	if empty {
		return nil, newEmptyManifest(dir), nil
	}

	return repo, nil, nil
}
