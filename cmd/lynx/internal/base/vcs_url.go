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

// ParseVCSUrl parses Version Control System (VCS) repository URL.
// Reference https://github.com/golang/go/blob/master/src/cmd/go/internal/vcs/vcs.go
// See https://go-review.googlesource.com/c/go/+/12226/
// Git URL definition reference https://git-scm.com/docs/git-clone#_git_urls
// Parameter repo is the repository URL string to parse.
// Returns the parsed *url.URL instance and any errors that may occur.
func ParseVCSUrl(repo string) (*url.URL, error) {
	var (
		repoURL *url.URL
		err     error
	)

	// Check if it's SCP-style URL syntax
	if m := scpSyntaxRe.FindStringSubmatch(repo); m != nil {
		// If SCP-style syntax is matched, convert it to standard URL format.
		// For example, "git@github.com:user/repo" will be converted to "ssh://git@github.com/user/repo".
		repoURL = &url.URL{
			Scheme: "ssh",
			User:   url.User(m[1]),
			Host:   m[2],
			Path:   m[3],
		}
	} else {
		// If it's not SCP-style syntax, ensure URL contains "//"
		if !strings.Contains(repo, "//") {
			repo = "//" + repo
		}
		// Handle URLs starting with "//git@", convert to "ssh:" protocol
		if strings.HasPrefix(repo, "//git@") {
			repo = "ssh:" + repo
		} else if strings.HasPrefix(repo, "//") {
			// Handle URLs starting with "//", convert to "https:" protocol
			repo = "https:" + repo
		}
		// Use standard library's url.Parse function to parse URL
		repoURL, err = url.Parse(repo)
		if err != nil {
			return nil, err
		}
	}

	// Check if the parsed URL protocol is in the supported protocol list
	// Also check insecure protocols, as this function is only used to report repository URL status
	for _, s := range scheme {
		if repoURL.Scheme == s {
			return repoURL, nil
		}
	}
	// If protocol is not supported, return error
	return nil, errors.New("unable to parse repo url")
}
