// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gerrit provides library functions for interacting with the
// gerrit code review system.
package gerrit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/collect"
	"fuchsia.googlesource.com/jiri/envvar"
)

var (
	autosubmitRE    = regexp.MustCompile("AutoSubmit")
	remoteRE        = regexp.MustCompile("remote:[^\n]*")
	multiPartRE     = regexp.MustCompile(`MultiPart:\s*(\d+)\s*/\s*(\d+)`)
	presubmitTestRE = regexp.MustCompile(`PresubmitTest:\s*(.*)`)

	queryParameters = []string{"CURRENT_REVISION", "CURRENT_COMMIT", "CURRENT_FILES", "LABELS", "DETAILED_ACCOUNTS"}
)

// Comment represents a single inline file comment.
type Comment struct {
	Line    int    `json:"line,omitempty"`
	Message string `json:"message,omitempty"`
}

// Review represents a Gerrit review. For more details, see:
// http://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#review-input
type Review struct {
	Message  string               `json:"message,omitempty"`
	Labels   map[string]string    `json:"labels,omitempty"`
	Comments map[string][]Comment `json:"comments,omitempty"`
}

// CLOpts records the review options.
type CLOpts struct {
	// Autosubmit determines if the CL should be auto-submitted when it
	// meets the submission rules.
	Autosubmit bool
	// Ccs records a list of email addresses to cc on the CL.
	Ccs []string
	// Draft determines if this CL is a draft.
	Draft bool
	// Edit determines if the user should be prompted to edit the commit
	// message when the CL is exported to Gerrit.
	Edit bool
	// Remote identifies the Gerrit remote that this CL will be pushed to
	Remote string
	// Presubmit determines what presubmit tests to run.
	Presubmit PresubmitTestType
	// RemoteBranch identifies the remote branch the CL pertains to.
	RemoteBranch string
	// Reviewers records a list of email addresses of CL reviewers.
	Reviewers []string
	// Topic records the CL topic.
	Topic string
	// Verify controls whether git pre-push hooks should be run before uploading.
	Verify bool
	//Ref to upload. Default is HEAD
	RefToUpload string
}

// Gerrit records a hostname of a Gerrit instance.
type Gerrit struct {
	host   *url.URL
	jirix  *jiri.X
	useSso bool
}

// New is the Gerrit factory.
func New(jirix *jiri.X, host *url.URL, useSso bool) *Gerrit {
	h := *host // copy host
	if useSso {
		h.Scheme = "http"
		h.Host = strings.Replace(h.Host, "googlesource.com", "git.corp.google.com", 1)
	}
	return &Gerrit{
		host:   &h,
		jirix:  jirix,
		useSso: useSso,
	}
}

// The following types reflect the schema Gerrit uses to represent
// CLs.
type CLList []Change
type CLRefMap map[string]Change
type Change struct {
	// CL data.
	Change_id        string
	Current_revision string
	Project          string
	Topic            string
	Branch           string
	Revisions        Revisions
	Subject          string
	Number           int `json:"_number"`
	Owner            Owner
	Labels           map[string]map[string]interface{}
	Submitted        string

	// Custom labels.
	AutoSubmit    bool
	MultiPart     *MultiPartCLInfo
	PresubmitTest PresubmitTestType
}
type Revisions map[string]Revision
type Revision struct {
	Fetch  `json:"fetch"`
	Commit `json:"commit"`
	Files  `json:"files"`
}

type RelatedChange struct {
	Change_id string
}
type RelatedChanges struct {
	Changes []RelatedChange
}

type Fetch struct {
	Http `json:"http"`
}
type Http struct {
	Ref string
}
type Parent struct {
	Commit string
}
type Commit struct {
	Parents []Parent
	Message string
}
type Owner struct {
	Name  string
	Email string
}
type Files map[string]struct{}
type ChangeError struct {
	Err error
	CL  Change
}

func (ce *ChangeError) Error() string {
	return ce.Err.Error()
}

func NewChangeError(cl Change, err error) *ChangeError {
	return &ChangeError{err, cl}
}

func (c Change) Reference() string {
	return c.Revisions[c.Current_revision].Fetch.Http.Ref
}

func (c Change) OwnerEmail() string {
	return c.Owner.Email
}

type PresubmitTestType string

const (
	PresubmitTestTypeNone PresubmitTestType = "none"
	PresubmitTestTypeAll  PresubmitTestType = "all"
)

func PresubmitTestTypes() []string {
	return []string{string(PresubmitTestTypeNone), string(PresubmitTestTypeAll)}
}

// parseQueryResults parses a list of Gerrit ChangeInfo entries (json
// result of a query) and returns a list of Change entries.
func parseQueryResults(reader io.Reader) (CLList, error) {
	r := bufio.NewReader(reader)

	// The first line of the input is the XSSI guard
	// ")]}'". Getting rid of that.
	if _, err := r.ReadSlice('\n'); err != nil {
		return nil, err
	}

	// Parse the remaining input to construct a slice of Change objects
	// to return.
	var changes CLList
	if err := json.NewDecoder(r).Decode(&changes); err != nil {
		return nil, fmt.Errorf("Decode() failed: %v", err)
	}

	newChanges := CLList{}
	for _, change := range changes {
		clMessage := change.Revisions[change.Current_revision].Commit.Message
		multiPartCLInfo, err := parseMultiPartMatch(clMessage)
		if err != nil {
			return nil, err
		}
		if multiPartCLInfo != nil {
			multiPartCLInfo.Topic = change.Topic
		}
		change.MultiPart = multiPartCLInfo
		change.PresubmitTest = parsePresubmitTestType(clMessage)
		change.AutoSubmit = autosubmitRE.FindStringSubmatch(clMessage) != nil
		newChanges = append(newChanges, change)
	}
	return newChanges, nil
}

// parseMultiPartMatch uses multiPartRE (a pattern like: MultiPart: 1/3) to match the given string.
func parseMultiPartMatch(match string) (*MultiPartCLInfo, error) {
	matches := multiPartRE.FindStringSubmatch(match)
	if matches != nil {
		index, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("Atoi(%q) failed: %v", matches[1], err)
		}
		total, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil, fmt.Errorf("Atoi(%q) failed: %v", matches[2], err)
		}
		return &MultiPartCLInfo{
			Index: index,
			Total: total,
		}, nil
	}
	return nil, nil
}

// parsePresubmitTestType uses presubmitTestRE to match the given string and
// returns the presubmit test type.
func parsePresubmitTestType(match string) PresubmitTestType {
	ret := PresubmitTestTypeAll
	matches := presubmitTestRE.FindStringSubmatch(match)
	if matches != nil {
		switch matches[1] {
		case string(PresubmitTestTypeNone):
			ret = PresubmitTestTypeNone
		case string(PresubmitTestTypeAll):
			ret = PresubmitTestTypeAll
		}
	}
	return ret
}

func makeHttpRequest(url string, cred *credentials) (io.Reader, func() error, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("NewRequest(GET, %q) failed: %s", url, err)
	}
	req.Header.Add("Accept", "application/json")
	// We ignore all errors when obtaining credentials since not every host requires them.
	if cred != nil {
		req.SetBasicAuth(cred.username, cred.password)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("Do(%v) failed: %s", req, err)
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, nil, fmt.Errorf("Query:Do(%v) failed: %s", req, res.StatusCode)
	}
	cleanup := func() error {
		return res.Body.Close()
	}
	return res.Body, cleanup, nil
}

func (g *Gerrit) makeRequest(url string, cred *credentials) (io.Reader, func() error, error) {
	if !g.useSso {
		return makeHttpRequest(url, cred)
	}

	if _, err := exec.LookPath("git-remote-persistent-https"); err != nil {
		return nil, nil, fmt.Errorf("cannot find executable 'git-remote-persistent-https', can't make sso request")
	}
	if _, err := exec.LookPath("curl"); err != nil {
		return nil, nil, fmt.Errorf("cannot find executable 'curl', can't make sso request")
	}
	type SsoCred struct {
		cookieFile string
		proxy      string
		err        error
		cmd        *exec.Cmd
	}
	c := make(chan SsoCred, 1)
	command := exec.Command("git-remote-persistent-https", "-print_config", url)
	defer func() {
		if command.Process != nil {
			command.Process.Kill()
		}
	}()
	go func() {
		cookieprefix := "http.cookiefile="
		proxyprefix := "http.proxy="
		var stdout, stderr bytes.Buffer
		command.Stdin = os.Stdin
		command.Stdout = &stdout
		command.Stderr = &stderr
		command.Env = envvar.MapToSlice(g.jirix.Env())
		g.jirix.Logger.Tracef("Run: git-remote-persistent-https -print_config %q", url)
		g.jirix.Logger.Infof("Trying to get sso cookie.")
		ssoCred := SsoCred{}
		if err := command.Start(); err != nil {
			ssoCred.err = err
			c <- ssoCred
			return
		}
		ticker := time.NewTicker(time.Millisecond * 100)
		go func() {
			ssoCred := SsoCred{}
			maxTicks := 100 // 10 seconds
			i := 0
			for _ = range ticker.C {
				i++
				txt := stdout.String()
				lines := strings.Split(txt, "\n")
				for _, line := range lines {
					if ssoCred.cookieFile == "" && strings.HasPrefix(line, cookieprefix) {
						ssoCred.cookieFile = line[len(cookieprefix):]
					}
					if ssoCred.proxy == "" && strings.HasPrefix(line, proxyprefix) {
						ssoCred.proxy = line[len(proxyprefix):]
					}
				}
				if ssoCred.cookieFile != "" && ssoCred.proxy != "" {
					ticker.Stop()
					c <- ssoCred
					return
				}
				if i >= maxTicks {
					ticker.Stop()
					g.jirix.Logger.Infof("kill")
					ssoCred.err = fmt.Errorf("Cannot find proper credentials, please run:\n%s\nand report bug", g.jirix.Color.Green("git-remote-persistent-https -print_config %q", url))
					c <- ssoCred
					command.Process.Kill()
					return
				}
			}
		}()
		// Leave it open, as closing it might delete cookie jar
		if err := command.Wait(); err != nil {
			ticker.Stop()
			ssoCred.err = fmt.Errorf("Error running  'git-remote-persistent-https': %s\nError msg:\n%s\n", err, stderr.String())
			c <- ssoCred
		}
		g.jirix.Logger.Tracef("killed")
	}()
	ssoCred := <-c
	if ssoCred.err != nil {
		return nil, nil, ssoCred.err
	}
	curlCommand := exec.Command("curl", "--fail", "--proxy", ssoCred.proxy, "--cookie", ssoCred.cookieFile, "--cookie-jar", ssoCred.cookieFile, "--location", url)
	var stdout, stderr bytes.Buffer
	curlCommand.Stdout = &stdout
	curlCommand.Stderr = &stderr
	curlCommand.Env = envvar.MapToSlice(g.jirix.Env())
	g.jirix.Logger.Tracef("Run: curl on %q", url)
	if err := curlCommand.Run(); err != nil {
		return nil, nil, fmt.Errorf("Error running curl: %s\n Error msg:\n%s\n", err, stderr.String())
	}
	return &stdout, nil, nil
}

// Query returns a list of QueryResult entries matched by the given
// Gerrit query string from the given Gerrit instance. The result is
// sorted by the last update time, most recently updated to oldest
// updated.
//
// See the following links for more details about Gerrit search syntax:
// - https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#list-changes
// - https://gerrit-review.googlesource.com/Documentation/user-search.html
func (g *Gerrit) Query(query string) (_ CLList, e error) {
	u, err := url.Parse(g.host.String())
	if err != nil {
		return nil, err
	}
	u.Path = "/changes/"
	cred, _ := hostCredentials(g.jirix, g.host)
	if cred != nil {
		// Gerrit requires prefixing the endpoint URL with /a/ for authentication.
		u.Path = "/a" + u.Path
	}
	v := url.Values{}
	v.Set("q", query)
	for _, o := range queryParameters {
		v.Add("o", o)
	}
	u.RawQuery = v.Encode()
	url := u.String()

	body, cleanup, err := g.makeRequest(url, cred)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer collect.Error(func() error { return cleanup() }, &e)
	}
	return parseQueryResults(body)
}

func (g *Gerrit) ListOpenChangesByTopic(topic string) (CLList, error) {
	return g.Query("topic:\"" + topic + "\" status:open")
}

func (g *Gerrit) ListChangesByCommit(commit string) (CLList, error) {
	return g.Query(fmt.Sprintf("commit:%s", commit))
}

// GetChange returns a Change object for the given changeId number.
func (g *Gerrit) GetChange(changeNumber int) (*Change, error) {
	clList, err := g.Query(fmt.Sprintf("%d", changeNumber))
	if err != nil {
		return nil, err
	}
	if len(clList) == 0 {
		return nil, fmt.Errorf("Query for change '%d' returned no results", changeNumber)
	}
	if len(clList) > 1 {
		// Based on cursory testing with Gerrit, I don't expect this to ever happen, but in
		// case it does, I'm raising an error to inspire investigation. -- lanechr
		return nil, fmt.Errorf("Too many changes returned for query '%d'", changeNumber)
	}
	return &clList[0], nil
}

func (g *Gerrit) GetRelatedChanges(changeNumber int, revisionId string) (*RelatedChanges, error) {
	u, err := url.Parse(g.host.String())
	if err != nil {
		return nil, err
	}
	u.Path = fmt.Sprintf("/changes/%d/revisions/%s/related", changeNumber, revisionId)
	cred, _ := hostCredentials(g.jirix, g.host)
	if cred != nil {
		// Gerrit requires prefixing the endpoint URL with /a/ for authentication.
		u.Path = "/a" + u.Path
	}
	url := u.String()

	body, cleanup, err := g.makeRequest(url, cred)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	r := bufio.NewReader(body)

	// The first line of the input is the XSSI guard
	// ")]}'". Getting rid of that.
	if _, err := r.ReadSlice('\n'); err != nil {
		return nil, err
	}

	// Parse the remaining input to construct a slice of Change objects
	// to return.
	var rc RelatedChanges
	if err := json.NewDecoder(r).Decode(&rc); err != nil {
		return nil, fmt.Errorf("Decode() failed: %v", err)
	}
	return &rc, nil
}

func (g *Gerrit) GetChangeByID(changeID string) (*Change, error) {
	clList, err := g.Query(fmt.Sprintf("%s", changeID))
	if err != nil {
		return nil, err
	}
	if len(clList) == 0 {
		return nil, nil
	}
	if len(clList) > 1 {
		// Based on cursory testing with Gerrit, I don't expect this to ever happen, but in
		// case it does, I'm raising an error to inspire investigation. -- lanechr
		return nil, fmt.Errorf("Too many changes returned for query '%s'", changeID)
	}
	return &clList[0], nil
}

func (g *Gerrit) GetChangeURL(changeNumber int) string {
	return fmt.Sprintf("%s/c/%d", g.host, changeNumber)
}

// formatParams formats parameters of a change list.
func formatParams(params []string, key string) []string {
	var keyedParams []string
	for _, param := range params {
		keyedParams = append(keyedParams, key+"="+param)
	}
	return keyedParams
}

// Reference inputs CL options and returns a matching string
// representation of a Gerrit reference.
func Reference(opts CLOpts) string {
	var ref string
	if opts.Draft {
		ref = "refs/drafts/" + opts.RemoteBranch
	} else {
		ref = "refs/for/" + opts.RemoteBranch
	}
	var params []string
	params = append(params, formatParams(opts.Reviewers, "r")...)
	params = append(params, formatParams(opts.Ccs, "cc")...)
	if opts.Topic != "" {
		params = append(params, "topic="+opts.Topic)
	}
	if len(params) > 0 {
		ref = ref + "%" + strings.Join(params, ",")
	}
	return ref
}

type PushError struct {
	Args        []string
	Output      string
	ErrorOutput string
}

func (ge PushError) Error() string {
	result := "'git "
	result += strings.Join(ge.Args, " ")
	result += "' failed:\n"
	result += ge.ErrorOutput
	return result
}

// Push pushes the current branch to Gerrit.
func Push(jirix *jiri.X, dir string, clOpts CLOpts) error {
	refToUpload := "HEAD"
	if clOpts.RefToUpload != "" {
		refToUpload = clOpts.RefToUpload
	}
	refspec := refToUpload + ":" + Reference(clOpts)
	args := []string{"push", clOpts.Remote, refspec}
	// TODO(jamesr): This should really reuse gitutil/git.go's Push which knows
	// how to set this option but doesn't yet know how to pipe stdout/stderr the way
	// this function wants.
	if clOpts.Verify {
		args = append(args, "--verify")
	} else {
		args = append(args, "--no-verify")
	}
	var stdout, stderr bytes.Buffer
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Stdin = os.Stdin
	command.Stdout = &stdout
	command.Stderr = &stderr
	env := jirix.Env()
	command.Env = envvar.MapToSlice(env)
	if err := command.Run(); err != nil {
		return PushError{args, stdout.String(), stderr.String()}
	}
	for _, line := range strings.Split(stderr.String(), "\n") {
		if remoteRE.MatchString(line) {
			fmt.Println(line)
		}
	}
	return nil
}

// ParseRefString parses the cl and patchset number from the given ref string.
func ParseRefString(ref string) (int, int, error) {
	parts := strings.Split(ref, "/")
	if expected, got := 5, len(parts); expected != got {
		return -1, -1, fmt.Errorf("unexpected number of %q parts: expected %v, got %v", ref, expected, got)
	}
	cl, err := strconv.Atoi(parts[3])
	if err != nil {
		return -1, -1, fmt.Errorf("Atoi(%q) failed: %v", parts[3], err)
	}
	patchset, err := strconv.Atoi(parts[4])
	if err != nil {
		return -1, -1, fmt.Errorf("Atoi(%q) failed: %v", parts[4], err)
	}
	return cl, patchset, nil
}
