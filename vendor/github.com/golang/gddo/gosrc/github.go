// Copyright 2013 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

package gosrc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func init() {
	addService(&service{
		pattern:         regexp.MustCompile(`^github\.com/(?P<owner>[a-z0-9A-Z_.\-]+)/(?P<repo>[a-z0-9A-Z_.\-]+)(?P<dir>/.*)?$`),
		prefix:          "github.com/",
		get:             getGitHubDir,
		getPresentation: getGitHubPresentation,
		getProject:      getGitHubProject,
	})

	addService(&service{
		pattern: regexp.MustCompile(`^gist\.github\.com/(?P<gist>[a-z0-9A-Z_.\-]+)\.git$`),
		prefix:  "gist.github.com/",
		get:     getGistDir,
	})
}

var (
	gitHubRawHeader     = http.Header{"Accept": {"application/vnd.github-blob.raw"}}
	gitHubPreviewHeader = http.Header{"Accept": {"application/vnd.github.preview"}}
	ownerRepoPat        = regexp.MustCompile(`^https://api.github.com/repos/([^/]+/[^/]+)/`)
)

type githubCommit struct {
	ID     string `json:"sha"`
	Commit struct {
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

func gitHubError(resp *http.Response) error {
	var e struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&e); err == nil {
		return &RemoteError{resp.Request.URL.Host, fmt.Errorf("%d: %s (%s)", resp.StatusCode, e.Message, resp.Request.URL.String())}
	}
	return &RemoteError{resp.Request.URL.Host, fmt.Errorf("%d: (%s)", resp.StatusCode, resp.Request.URL.String())}
}

func getGitHubDir(ctx context.Context, client *http.Client, match map[string]string, savedEtag string) (*Directory, error) {

	c := &httpClient{client: client, errFn: gitHubError}

	var repo struct {
		Fork          bool      `json:"fork"`
		Stars         int       `json:"stargazers_count"`
		CreatedAt     time.Time `json:"created_at"`
		PushedAt      time.Time `json:"pushed_at"`
		DefaultBranch string    `json:"default_branch"`
	}

	if _, err := c.getJSON(ctx, expand("https://api.github.com/repos/{owner}/{repo}", match), &repo); err != nil {
		return nil, err
	}

	status := Active
	var commits []*githubCommit
	u := expand("https://api.github.com/repos/{owner}/{repo}/commits", match)
	if match["dir"] != "" {
		u += fmt.Sprintf("?path=%s", url.QueryEscape(match["dir"]))
	}
	if _, err := c.getJSON(ctx, u, &commits); err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, NotFoundError{Message: "package directory changed or removed"}
	}

	lastCommitted := commits[0].Commit.Committer.Date
	if lastCommitted.Add(ExpiresAfter).Before(time.Now()) {
		status = NoRecentCommits
	} else if repo.Fork {
		if repo.PushedAt.Before(repo.CreatedAt) {
			status = DeadEndFork
		} else if isQuickFork(commits, repo.CreatedAt) {
			status = QuickFork
		}
	}
	if commits[0].ID == savedEtag {
		return nil, NotModifiedError{
			Since:  lastCommitted,
			Status: status,
		}
	}

	var contents []*struct {
		Type    string
		Name    string
		GitURL  string `json:"git_url"`
		HTMLURL string `json:"html_url"`
	}

	if _, err := c.getJSON(ctx, expand("https://api.github.com/repos/{owner}/{repo}/contents{dir}", match), &contents); err != nil {
		// The GitHub content API returns array values for directories
		// and object values for files. If there's a type mismatch at
		// the beginning of the response, then assume that the path is
		// for a file.
		if e, ok := err.(*json.UnmarshalTypeError); ok && e.Offset == 1 {
			return nil, NotFoundError{Message: "Not a directory"}
		}
		return nil, err
	}

	if len(contents) == 0 {
		return nil, NotFoundError{Message: "No files in directory."}
	}

	if err := validateGitHubProjectName(contents[0].GitURL, match); err != nil {
		return nil, err
	}

	var files []*File
	var dataURLs []string
	var subdirs []string

	for _, item := range contents {
		switch {
		case item.Type == "dir":
			if isValidPathElement(item.Name) {
				subdirs = append(subdirs, item.Name)
			}
		case isDocFile(item.Name):
			files = append(files, &File{Name: item.Name, BrowseURL: item.HTMLURL})
			dataURLs = append(dataURLs, item.GitURL)
		}
	}

	c.header = gitHubRawHeader
	if err := c.getFiles(ctx, dataURLs, files); err != nil {
		return nil, err
	}

	browseURL := expand("https://github.com/{owner}/{repo}", match)
	if match["dir"] != "" {
		match["tag"] = repo.DefaultBranch // TODO: This doesn't respect "go1" tag/branch special case.
		browseURL = expand("https://github.com/{owner}/{repo}/tree/{tag}{dir}", match)
	}

	return &Directory{
		BrowseURL:      browseURL,
		Etag:           commits[0].ID,
		Files:          files,
		LineFmt:        "%s#L%d",
		ProjectName:    match["repo"],
		ProjectRoot:    expand("github.com/{owner}/{repo}", match),
		ProjectURL:     expand("https://github.com/{owner}/{repo}", match),
		Subdirectories: subdirs,
		VCS:            "git",
		Status:         status,
		Fork:           repo.Fork,
		Stars:          repo.Stars,
	}, nil
}

// validateGitHubProjectName checks if the requested owner and repo names match
// the case of the canonical names returned by Github. GitHub owner and repo names
// are case-insensitive so they must be normalized to the canonical casing.
//
// Returns a not found error if the names are the same, but have different case.
// Returns nil if the names match exactly, or if the names are different,
// which will happen if the url is a github redirect to a new project name.
func validateGitHubProjectName(canonicalName string, requestMatch map[string]string) error {
	m := ownerRepoPat.FindStringSubmatch(canonicalName)
	if m == nil {
		return nil
	}
	requestedName := requestMatch["owner"] + "/" + requestMatch["repo"]
	if m[1] == requestedName || !strings.EqualFold(m[1], requestedName) {
		return nil
	}
	return NotFoundError{
		Message:  "Github import path has incorrect case.",
		Redirect: fmt.Sprintf("github.com/%s%s", m[1], requestMatch["dir"]),
	}
}

// isQuickFork reports whether the repository is a "quick fork":
// it has fewer than 3 commits, all within a week of the repo creation, createdAt.
// Commits must be in reverse chronological order by Commit.Committer.Date.
func isQuickFork(commits []*githubCommit, createdAt time.Time) bool {
	oneWeekOld := createdAt.Add(7 * 24 * time.Hour)
	if oneWeekOld.After(time.Now()) {
		return false // a newborn baby of a repository
	}
	n := 0
	for _, commit := range commits {
		if commit.Commit.Committer.Date.After(oneWeekOld) {
			return false
		}
		if commit.Commit.Committer.Date.Before(createdAt) {
			break
		}
		n++
	}
	return n < 3
}

func getGitHubPresentation(ctx context.Context, client *http.Client, match map[string]string) (*Presentation, error) {
	c := &httpClient{client: client, header: gitHubRawHeader}

	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if _, err := c.getJSON(ctx, expand("https://api.github.com/repos/{owner}/{repo}", match), &repo); err != nil {
		return nil, err
	}
	branch := repo.DefaultBranch

	p, err := c.getBytes(ctx, expand("https://api.github.com/repos/{owner}/{repo}/contents{dir}/{file}", match))
	if err != nil {
		return nil, err
	}

	apiBase, err := url.Parse(expand("https://api.github.com/repos/{owner}/{repo}/contents{dir}/", match))
	if err != nil {
		return nil, err
	}
	rawBase, err := url.Parse(expand("https://raw.githubusercontent.com/{owner}/{repo}/{0}{dir}/", match, branch))
	if err != nil {
		return nil, err
	}

	c.header = gitHubRawHeader

	b := &presBuilder{
		data:     p,
		filename: match["file"],
		fetch: func(fnames []string) ([]*File, error) {
			var files []*File
			var dataURLs []string
			for _, fname := range fnames {
				u, err := apiBase.Parse(fname)
				if err != nil {
					return nil, err
				}
				u.RawQuery = apiBase.RawQuery
				files = append(files, &File{Name: fname})
				dataURLs = append(dataURLs, u.String())
			}
			err := c.getFiles(ctx, dataURLs, files)
			return files, err
		},
		resolveURL: func(fname string) string {
			u, err := rawBase.Parse(fname)
			if err != nil {
				return "/notfound"
			}
			if strings.HasSuffix(fname, ".svg") {
				u.Host = "rawgithub.com"
			}
			return u.String()
		},
	}

	return b.build()
}

// GetGitHubUpdates returns the full names ("owner/repo") of recently pushed GitHub repositories.
// by pushedAfter.
func GetGitHubUpdates(ctx context.Context, client *http.Client, pushedAfter string) (maxPushedAt string, names []string, err error) {
	c := httpClient{client: client, header: gitHubPreviewHeader}

	if pushedAfter == "" {
		pushedAfter = time.Now().Add(-24 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	}
	u := "https://api.github.com/search/repositories?order=asc&sort=updated&q=fork:true+language:Go+pushed:>" + pushedAfter
	var updates struct {
		Items []struct {
			FullName string `json:"full_name"`
			PushedAt string `json:"pushed_at"`
		}
	}
	_, err = c.getJSON(ctx, u, &updates)
	if err != nil {
		return pushedAfter, nil, err
	}

	maxPushedAt = pushedAfter
	for _, item := range updates.Items {
		names = append(names, item.FullName)
		if item.PushedAt > maxPushedAt {
			maxPushedAt = item.PushedAt
		}
	}
	return maxPushedAt, names, nil
}

func getGitHubProject(ctx context.Context, client *http.Client, match map[string]string) (*Project, error) {
	c := &httpClient{client: client, errFn: gitHubError}

	var repo struct {
		Description string
	}

	if _, err := c.getJSON(ctx, expand("https://api.github.com/repos/{owner}/{repo}", match), &repo); err != nil {
		return nil, err
	}

	return &Project{
		Description: repo.Description,
	}, nil
}

func getGistDir(ctx context.Context, client *http.Client, match map[string]string, savedEtag string) (*Directory, error) {
	c := &httpClient{client: client, errFn: gitHubError}

	var gist struct {
		Files map[string]struct {
			Content string
		}
		HTMLURL string `json:"html_url"`
		History []struct {
			Version string
		}
	}

	if _, err := c.getJSON(ctx, expand("https://api.github.com/gists/{gist}", match), &gist); err != nil {
		return nil, err
	}

	if len(gist.History) == 0 {
		return nil, NotFoundError{Message: "History not found."}
	}
	commit := gist.History[0].Version

	if commit == savedEtag {
		return nil, NotModifiedError{}
	}

	var files []*File

	for name, file := range gist.Files {
		if isDocFile(name) {
			files = append(files, &File{
				Name:      name,
				Data:      []byte(file.Content),
				BrowseURL: gist.HTMLURL + "#file-" + strings.Replace(name, ".", "-", -1),
			})
		}
	}

	return &Directory{
		BrowseURL:      gist.HTMLURL,
		Etag:           commit,
		Files:          files,
		LineFmt:        "%s-L%d",
		ProjectName:    match["gist"],
		ProjectRoot:    expand("gist.github.com/{gist}.git", match),
		ProjectURL:     gist.HTMLURL,
		Subdirectories: nil,
		VCS:            "git",
	}, nil
}
