package presenter

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/shurcooL/go-goon"
	"github.com/shurcooL/go/gists/gist7480523"
	"github.com/sourcegraph/go-vcs/vcs"
	"github.com/sourcegraph/vcsstore/vcsclient"
)

type codeGooglePresenter struct {
	repo *gist7480523.GoPackageRepo

	comparison codeGoogleComparison
}

func newCodeGooglePresenter(repo *gist7480523.GoPackageRepo) Presenter {
	return &codeGooglePresenter{
		repo:       repo,
		comparison: newCodeGoogleComparison(repo),
	}
}

func (this codeGooglePresenter) Repo() *gist7480523.GoPackageRepo {
	return this.repo
}
func (this codeGooglePresenter) HomePage() *template.URL {
	url := template.URL("https://" + this.repo.RepoImportPath())
	return &url
}
func (_ codeGooglePresenter) Image() template.URL {
	return "https://github.com/images/gravatars/gravatar-user-420.png"
}
func (this codeGooglePresenter) Changes() <-chan Change {
	if this.comparison.err != nil {
		return nil
	}
	out := make(chan Change)
	go func() {
		foundLocalRev := false
		for _, commit := range this.comparison.commits {
			// Break out when/if we reach the current local revision.
			if commit.ID == vcs.CommitID(this.repo.GoPackages()[0].Dir.Repo.VcsLocal.LocalRev) {
				foundLocalRev = true
				break
			}
			out <- change{
				message: firstParagraph(commit.Message),
				url:     codeGoogleCommitUrl(this.repo, commit.ID),
			}
		}
		if !foundLocalRev {
			out <- change{message: "... (there may be more changes, not shown)"}
		}
		close(out)
	}()
	return out
}

// firstParagraph returns the first paragraph of a string.
func firstParagraph(s string) string {
	if index := strings.Index(s, "\n\n"); index != -1 {
		return s[:index]
	}
	return s
}

func codeGoogleCommitUrl(repo *gist7480523.GoPackageRepo, commitId vcs.CommitID) template.URL {
	repoNameElements := strings.Split(strings.TrimPrefix(repo.RepoImportPath(), "code.google.com/p/"), ".")
	values := url.Values{
		"r": {string(commitId)},
	}
	if len(repoNameElements) >= 2 {
		values["repo"] = []string{repoNameElements[1]}
	}
	url := url.URL{
		Scheme:   "https",
		Host:     "code.google.com",
		Path:     "/p/" + repoNameElements[0] + "/source/detail",
		RawQuery: values.Encode(),
	}
	return template.URL(url.String())
}

// ---

var sg = vcsclient.New(&url.URL{Scheme: "http", Host: "vcsstore.sourcegraph.com"}, nil)

type codeGoogleComparison struct {
	commits []*vcs.Commit
	err     error
}

func newCodeGoogleComparison(repo *gist7480523.GoPackageRepo) (c codeGoogleComparison) {
	cloneUrl, err := url.Parse("https://" + repo.RepoImportPath())
	if err != nil {
		c.err = err
		return
	}

	r, err := sg.Repository("hg", cloneUrl) // code.google.com/p/... repos are known to use Mercurial.
	if err != nil {
		c.err = err
		return
	}

	var debug = false
	commitId, err := r.ResolveRevision(repo.GoPackages()[0].Dir.Repo.VcsRemote.RemoteRev)
	if err != nil {
		err1 := r.(vcsclient.RepositoryCloneUpdater).CloneOrUpdate(vcs.RemoteOpts{})
		fmt.Println("newCodeGoogleComparison: CloneOrUpdate:", err1)
		if err1 != nil {
			c.err = MultiError{err, err1}
			return
		}
		fmt.Println("newCodeGoogleComparison: DID NOT work on first try!!!")
		goon.DumpExpr(repo.GoPackages()[0].Dir.Repo.VcsRemote.RemoteRev)
		goon.DumpExpr(commitId)
		goon.DumpExpr(err.Error())
		goon.DumpExpr(err)
		debug = true
		commitId, err1 = r.ResolveRevision(repo.GoPackages()[0].Dir.Repo.VcsRemote.RemoteRev)
		if err1 != nil {
			c.err = MultiError{err, err1}
			return
		}
		fmt.Println("newCodeGoogleComparison: did work on second try!!!!!!!!")
		goon.DumpExpr(repo.GoPackages()[0].Dir.Repo.VcsRemote.RemoteRev)
		goon.DumpExpr(commitId)
	} else {
		fmt.Println("newCodeGoogleComparison: worked on first try")
	}

	c.commits, _, c.err = r.Commits(vcs.CommitsOptions{
		Head: commitId,
		N:    20, // Cap for now. TODO: Support arbtirary second revision to go until.
	})
	if debug {
		goon.DumpExpr(c)
	}
	return
}

// ---

type MultiError []error

func (me MultiError) Error() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%d errors:\n", len(me))
	for _, err := range me {
		fmt.Fprintln(&buf, err.Error())
	}
	return buf.String()
}
