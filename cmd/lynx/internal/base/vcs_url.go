package base

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// scpSyntaxRe is used to match SCP-style URL syntax, such as "git@github.com:user/repo".
var (
	scpSyntaxRe = regexp.MustCompile(`^(\w+)@([\w.-]+):(.*)$`)
	// scheme defines the list of supported Version Control System (VCS) URL protocols.
	scheme = []string{"git", "https", "http", "git+ssh", "ssh", "file", "ftp", "ftps"}
)

// ParseVCSUrl parses a VCS repository URL, accepting both standard URLs and
// SCP-style git remotes, and validates the scheme against the supported set.
// Adapted from cmd/go's internal vcs handling:
//   - https://github.com/golang/go/blob/master/src/cmd/go/internal/vcs/vcs.go
//   - https://go-review.googlesource.com/c/go/+/12226/
//   - https://git-scm.com/docs/git-clone#_git_urls
func ParseVCSUrl(repo string) (*url.URL, error) {
	var (
		repoURL *url.URL
		err     error
	)

	// SCP-style syntax ("git@github.com:user/repo") has no scheme, so rewrite it
	// into an explicit ssh:// URL that url.Parse can handle.
	if m := scpSyntaxRe.FindStringSubmatch(repo); m != nil {
		repoURL = &url.URL{
			Scheme: "ssh",
			User:   url.User(m[1]),
			Host:   m[2],
			Path:   m[3],
		}
	} else {
		// Supply a scheme for bare host/path forms before parsing: "//git@..."
		// implies ssh, any other "//..." implies https.
		if !strings.Contains(repo, "//") {
			repo = "//" + repo
		}
		if strings.HasPrefix(repo, "//git@") {
			repo = "ssh:" + repo
		} else if strings.HasPrefix(repo, "//") {
			repo = "https:" + repo
		}
		repoURL, err = url.Parse(repo)
		if err != nil {
			return nil, err
		}
	}

	// Insecure schemes are accepted too: this only reports URL shape, not policy.
	for _, s := range scheme {
		if repoURL.Scheme == s {
			return repoURL, nil
		}
	}
	return nil, errors.New("unable to parse repo url")
}
