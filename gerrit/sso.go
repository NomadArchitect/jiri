package gerrit

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/gitutil"
	"fuchsia.googlesource.com/jiri/osutil"
	"golang.org/x/net/publicsuffix"
)

const (
	cookieHTTPOnlyPrefix = "#HttpOnly_"
	generatePasswordURL  = "https://%s/new-password"
)

var (
	ErrRedirectOnGerrit       = errors.New("got redirection while downloading file from gerrit server")
	ErrRedirectOnGerritSSO    = errors.New("got redirection while downloading file from gerrit server using SSO cookie")
	ErrCookieNotExist         = errors.New("cookie file not found")
	ErrSSOHelperNotExist      = errors.New("jirissohelper not found")
	ErrSSOCookieExpireInvalid = errors.New("SSO cookie is either invalid or expired, please run \"glogin\" and try again")
	ssoCookieAge              = 30 * 24 * time.Hour // cookie expiration time from HTTP response
	ssoCookieLife             = 20 * time.Hour      // actual cookie expiration time according to documentation
	ssoHelperPath             = ""
	ssoCookieCachePath        = ""
	bootstrapOnce             sync.Once
)

// The golang cookiejar implementation will remove the domain name, expiration
// time etc. from the cookies returned from Cookies(*url.URL) method (maybe for
// security reasons). The expiration time is crucial for caching the sso
// cookies, therefore we build our own cookiejar to save a copy of unstripped
// SSO cookie.
type ssoCookieJar struct {
	jar        http.CookieJar
	ssoCookies map[string]*http.Cookie
}

// BootstrapGerritSSO will setup cookie cache for SSO cookies and setup the
// path for jirissohelper. Due to security concerns, we cannot hard code sso
// cookie paths in jiri, instead, we invoke jirissohelper to get current path
// of SSO cookie. jirissohelper should be placed at the same directory of jiri
// binary. It will be fetched from cipd once we have cipd support landed.
func BootstrapGerritSSO(jirix *jiri.X) error {
	var e error
	bootstrapOnce.Do(func() {
		jiriPath, err := osutil.Executable()
		if err != nil {
			e = err
			return
		}
		jiriBinaryRoot := path.Dir(jiriPath)
		ssoHelperPath = path.Join(jiriBinaryRoot, "jirissohelper")
		ssoCookieCachePath = path.Join(jirix.RootMetaDir(), "jiricookies.txt")
	})
	return e
}

// newSSOCookieJar constructs a new ssoCookieJar instance. Based on golang
// cookiejar implementation, the error will always be nil.
func newSSOCookieJar() (*ssoCookieJar, error) {
	j, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	return &ssoCookieJar{
		jar:        j,
		ssoCookies: make(map[string]*http.Cookie),
	}, nil
}

// SetCookies overrides cookiejar.SetCookies method, which implements the
// SetCookies method of the http.CookieJar interface. It does nothing if
// the URL's scheme is not HTTP or HTTPS.
func (j *ssoCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	// Save/update SSO cookies
	for _, cookie := range cookies {
		if cookie.Name == "SSO" {
			if j.ssoCookies[u.Host] == nil || j.ssoCookies[u.Host].Expires.Before(cookie.Expires) {
				j.ssoCookies[u.Host] = cookie
			}
		}
	}
	j.jar.SetCookies(u, cookies)
}

// Cookies overrides cookiejar.Cookies method, which implements the Cookies
// method of the http.CookieJar interface. It returns an empty slice if the
// URL's scheme is not HTTP or HTTPS.
func (j *ssoCookieJar) Cookies(u *url.URL) (cookies []*http.Cookie) {
	return j.jar.Cookies(u)
}

// GetSSOCookie will return saved SSO cookie for url u. It will return nil
// if that cookie does not exist.
func (j *ssoCookieJar) GetSSOCookie(u *url.URL) (cookie *http.Cookie) {
	return j.ssoCookies[u.Host]
}

// FetchFile downloads a file and returns its content to a byte slice. It will
// return ErrRedirectOnGerrit if redirection is detected, which indicates that
// user authentication is required.
func FetchFile(gerritHost, path string) ([]byte, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	downloadPath := gerritHost + path
	resp, err := client.Get(downloadPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Check if there is an redirection
	if resp.StatusCode != http.StatusOK {
		if _, err := resp.Location(); err == nil {
			return nil, ErrRedirectOnGerrit
		}
		return nil, fmt.Errorf("expecting status code %d from %q, got %d ", http.StatusOK, downloadPath, resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

func fetchFileSSO(gerritHost, path string, jar http.CookieJar) ([]byte, error) {
	hostName := gerritHost[len("https://"):]
	client := &http.Client{
		Jar: jar,
	}
	downloadPath := gerritHost + path
	resp, err := client.Get(downloadPath)
	if err != nil {
		return nil, err
	}

	if resp.Request != nil {
		if resp.Request.URL.Host != hostName {
			// If final request have different hostname than gerritHost
			// It indicates gerrit SSO cookie is either invalid or expired
			return nil, ErrRedirectOnGerritSSO
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expecting status code %d from %q, got %d ", http.StatusOK, downloadPath, resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

// FetchFileSSO downloads a file from a gerrit host that requires SSO login
// and returns its content to a byte slice. Since it uses user's master SSO
// cookie, the scheme of the url should always be HTTPS, otherwise an error
// will be returned.
func FetchFileSSO(jirix *jiri.X, gerritHost, path string) ([]byte, error) {
	if err := BootstrapGerritSSO(jirix); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(gerritHost, "https://") {
		return nil, fmt.Errorf("Unsupported scheme for host %q", gerritHost)
	}
	hostName := gerritHost[len("https://"):]
	jar, err := LoadCookies(jirix, ssoCookieCachePath, hostName, false)
	if err != nil {
		return nil, err
	}
	data, err := fetchFileSSO(gerritHost, path, jar)
	if err == ErrRedirectOnGerritSSO {
		// The cached cookie might be expired eventhough it is not
		// marked as expired in the cache file, retry using master SSO
		// cookie
		jar, err = LoadCookies(jirix, ssoCookieCachePath, hostName, true)
		if err != nil {
			return nil, err
		}
		data, err = fetchFileSSO(gerritHost, path, jar)
		if err == ErrRedirectOnGerritSSO {
			// It generally means both gerrit SSO cookie and
			// master SSO cookies are both exipred, ask user to refresh
			// cookies
			return nil, ErrSSOCookieExpireInvalid
		}
	}
	if err != nil {
		return nil, err
	}
	if err := CacheCookies(ssoCookieCachePath, hostName, jar); err != nil {
		return nil, err
	}
	return data, nil
}

// UnmarshalNSCookieData parses the Netscape-format cookies from
// data and return a slice of golang cookies.
func UnmarshalNSCookieData(data []byte) ([]*http.Cookie, error) {
	var cookieBuf bytes.Buffer
	if _, err := cookieBuf.Write(data); err != nil {
		return nil, err
	}

	cookieScanner := bufio.NewScanner(&cookieBuf)
	returnList := make([]*http.Cookie, 0)
	for cookieScanner.Scan() {
		currLine := strings.TrimSpace(cookieScanner.Text())
		fields := strings.Fields(currLine)
		// Skip unrelated lines
		if len(fields) != 7 {
			continue
		}
		cookie := &http.Cookie{}
		if strings.HasPrefix(fields[0], cookieHTTPOnlyPrefix) {
			cookie.Domain = fields[0][len(cookieHTTPOnlyPrefix):]
			cookie.HttpOnly = true
		} else {
			cookie.Domain = fields[0]
		}
		cookie.Path = fields[2]
		secure, err := strconv.ParseBool(fields[3])
		if err != nil {
			return nil, err
		}
		cookie.Secure = secure
		timestamp, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil {
			return nil, err
		}
		cookie.Expires = time.Unix(timestamp, 0)
		cookie.Name = fields[5]
		cookie.Value = fields[6]
		returnList = append(returnList, cookie)
	}
	return returnList, nil
}

// MarshalNSCookieData packs the slice of golang cookies into
// the Netscape-format cookies.
func MarshalNSCookieData(cookies []*http.Cookie) ([]byte, error) {
	var cookieBuf bytes.Buffer
	if cookies == nil || len(cookies) == 0 {
		return cookieBuf.Bytes(), nil
	}

	for _, cookie := range cookies {
		var builder strings.Builder
		if cookie.HttpOnly {
			builder.WriteString(cookieHTTPOnlyPrefix + cookie.Domain)
		} else {
			builder.WriteString(cookie.Domain)
		}
		builder.WriteRune('\t')
		builder.WriteString("FALSE")
		builder.WriteRune('\t')
		if cookie.Path != "" {
			builder.WriteString(cookie.Path)
		} else {
			builder.WriteString("/")
		}
		builder.WriteRune('\t')
		if cookie.Secure {
			builder.WriteString("TRUE")
		} else {
			builder.WriteString("FALSE")
		}
		builder.WriteRune('\t')
		builder.WriteString(strconv.FormatInt(cookie.Expires.Unix(), 10))
		builder.WriteRune('\t')
		builder.WriteString(cookie.Name)
		builder.WriteRune('\t')
		builder.WriteString(cookie.Value)
		builder.WriteRune('\n')
		if _, err := cookieBuf.WriteString(builder.String()); err != nil {
			return nil, err
		}
	}
	return cookieBuf.Bytes(), nil
}

func loadJiriCookies(jiriCookiePath string) []*http.Cookie {
	jiriCookieData, err := ioutil.ReadFile(jiriCookiePath)
	if err != nil {
		return nil
	}

	cookies, err := UnmarshalNSCookieData(jiriCookieData)
	if err != nil {
		return nil
	}
	return cookies
}

// LoadCookies loads necessary cookies from various sources (master sso,
// gitcookies and cached jiricookies), returning a cookiejar that contains
// necessary cookies to login to the hostName. An error will be returned
// if no suitable cookie is found or if there is an I/O error.
func LoadCookies(jirix *jiri.X, jiriCookiePath, hostName string, forceUsingMasterSSO bool) (*ssoCookieJar, error) {
	cookieJar, err := newSSOCookieJar()

	// Read jiriCookiePath, it may have cached cookies for gerrit host
	if !forceUsingMasterSSO {
		cachedSSOCookies := loadJiriCookies(jiriCookiePath)
		var cachedSSOCookie *http.Cookie
		if cachedSSOCookies != nil {
			for _, cookie := range cachedSSOCookies {
				if cookie.Name == "SSO" && cookie.Domain == hostName {
					if cookie.Expires.After(time.Now()) {
						cachedSSOCookie = cookie
						break
					}
				}
			}
		}
		if cachedSSOCookie != nil {
			cookieJar.SetCookies(&url.URL{
				Scheme: "https",
				Host:   cachedSSOCookie.Domain,
				Path:   "/",
			}, []*http.Cookie{cachedSSOCookie})
			return cookieJar, nil
		}
	}

	// No cached/unexpired gerrit SSO cookie found, fall back to master SSO
	// cookie with git cookies. Read master SSO cookie
	ssoPath, err := getSSOCookiePath()
	if err != nil {
		if err == ErrCookieNotExist {
			return nil, ErrSSOCookieExpireInvalid
		}
		return nil, err
	}
	ssoCookie, err := readMasterSSOCookie(ssoPath)
	if err != nil {
		return nil, err
	}
	if loginRequired(ssoCookie) {
		return nil, ErrSSOCookieExpireInvalid
	}
	cookieJar.SetCookies(&url.URL{
		Scheme: "https",
		Host:   ssoCookie.Domain,
		Path:   "/",
	}, []*http.Cookie{ssoCookie})

	// Read .gitcookies
	gitCookiePath, err := getCookiePath(jirix)
	if err != nil {
		if err == ErrCookieNotExist {
			return nil, fmt.Errorf("git cookies not found, please follow the instructions at %q", fmt.Sprintf(generatePasswordURL, hostName))
		}
		return nil, err
	}
	gitCookieData, err := ioutil.ReadFile(gitCookiePath)
	if err != nil {
		return nil, err
	}
	cookies, err := UnmarshalNSCookieData(gitCookieData)

	// Looking for git cookies for gerrit Host
	var gerritGitCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Domain == hostName && cookie.Name == "o" && strings.HasPrefix(cookie.Value, "git") {
			gerritGitCookie = cookie
			break
		}
	}

	if gerritGitCookie == nil {
		return nil, fmt.Errorf("cookie for %q is not found in git cookies, please follow the instructions at %q", hostName, fmt.Sprintf(generatePasswordURL, hostName))
	}
	cookieJar.SetCookies(&url.URL{
		Scheme: "https",
		Host:   gerritGitCookie.Domain,
		Path:   "/",
	}, []*http.Cookie{gerritGitCookie})
	return cookieJar, nil
}

// CacheCookies saves the gerrit SSO cookie back jiriCookiePath file.
// As there is a limit on how many SSO cookies can be requested per hour,
// caching the gerrit SSO cookie allows jiri to avoid hitting the limiter.
func CacheCookies(jiriCookiePath, hostName string, cookiejar *ssoCookieJar) error {
	// Read the cache first
	var cookies []*http.Cookie
	cookies = loadJiriCookies(jiriCookiePath)
	if cookies == nil {
		cookies = make([]*http.Cookie, 0)
	}
	// Retrieve latest gerrit SSO cookie from jar
	latestSSOCookie := cookiejar.GetSSOCookie(&url.URL{
		Scheme: "https",
		Path:   "/",
		Host:   hostName,
	})
	if latestSSOCookie == nil {
		return errors.New("gerrit SSO cookie not found in cookie jar")
	}
	latestSSOCookie.Domain = hostName
	latestSSOCookie.Path = "/"
	var gerritSSOCookieExists bool
	for i, cookie := range cookies {
		if cookie.Name == latestSSOCookie.Name && cookie.Domain == latestSSOCookie.Domain {
			gerritSSOCookieExists = true
			// Only replace the cookie if the cached cookie expires earlier than latestSSOCookie in
			if cookie.Expires.Before(latestSSOCookie.Expires) {
				cookies[i] = latestSSOCookie
			}
			break
		}
	}

	if !gerritSSOCookieExists {
		cookies = append(cookies, latestSSOCookie)
	}

	jiriCookieData, err := MarshalNSCookieData(cookies)
	if err != nil {
		return err
	}

	tempFile, err := ioutil.TempFile(path.Dir(jiriCookiePath), ".jiricookie*")
	if err != nil {
		return err
	}
	_, err = tempFile.Write(jiriCookieData)
	if err != nil {
		tempFile.Close()
		return err
	}
	tempFile.Close()
	if err := os.Rename(tempFile.Name(), jiriCookiePath); err != nil {
		os.Remove(tempFile.Name())
		return err
	}
	return nil
}

func loginRequired(cookie *http.Cookie) bool {
	// The correct way to determine cookie expiration time
	// is to use protobuf to unmarshal ticket data in cookie
	// However, we don't want to expose ticket data structure here,
	// So we use a work around
	cookie.Expires = cookie.Expires.Add(-ssoCookieAge).Add(ssoCookieLife)
	if cookie.Expires.Before(time.Now()) {
		return true
	}
	return false
}

func getCookiePath(jirix *jiri.X) (string, error) {
	if cookieFilePath, err := gitutil.New(jirix).ConfigGetKey("http.cookiefile"); err == nil {
		cookieFilePath = strings.TrimSpace(cookieFilePath)
		if _, err := os.Stat(cookieFilePath); err != nil {
			if os.IsNotExist(err) {
				return "", ErrCookieNotExist
			}
			return "", err
		}
		return cookieFilePath, nil
	} else {
		return "", err
	}
}

func getSSOCookiePath() (string, error) {
	if _, err := os.Stat(ssoHelperPath); err != nil {
		if os.IsNotExist(err) {
			return "", ErrSSOHelperNotExist
		}
		return "", err
	}
	command := exec.Command(ssoHelperPath, "")
	var stdoutBuf, stderrBuf bytes.Buffer
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf
	if err := command.Run(); err != nil {
		if stderrBuf.Len() != 0 {
			// ssohelper returned an error message
			return "", fmt.Errorf("failed to run ssohelper, %v", strings.TrimSpace(stderrBuf.String()))
		} else {
			return "", fmt.Errorf("failed to run ssohelper, %v", err)
		}
	}
	ssoPath := strings.TrimSpace(stdoutBuf.String())
	if _, err := os.Stat(ssoPath); err != nil {
		if os.IsNotExist(err) {
			return "", ErrCookieNotExist
		}
		return "", err
	}
	return ssoPath, nil
}

func readMasterSSOCookie(path string) (*http.Cookie, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.SplitN(string(data), "\n", 2)
	cookie := http.Cookie{}

	for _, field := range strings.Split(lines[0], ",") {
		kv := strings.SplitN(field, "=", 2)
		switch kv[0] {
		case "domain":
			cookie.Domain = kv[1]
		case "name":
			cookie.Name = kv[1]
		case "path":
			cookie.Path = kv[1]
		case "value":
			cookie.Value = kv[1]
		case "expires":
			expires, err := strconv.ParseInt(kv[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid expires time in cookie: %v", err)
			}
			cookie.Expires = time.Unix(int64(expires), 0)
			cookie.RawExpires = kv[1]
		case "secure":
			if kv[1] != "True" {
				return nil, fmt.Errorf("secure value in cookie is not True")
			}
			cookie.Secure = true
		}
	}
	return &cookie, nil
}
