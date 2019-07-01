package mirror

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

var Config *Yaml
var initial bool
var protocal string

type Replaced struct {
	Old string `yaml:"old"`
	New string `yaml:"new"`
}

type Yaml struct {
	Host struct {
		Self  string `yaml:"self"`
		Proxy string `yaml:"proxy"`
	}
	ReplacedURLs []Replaced `yaml:"replaced_urls"`
	EnableSSL    bool       `yaml:"enable_ssl"`
	HandleCookie bool       `yaml:"handle_cookie"`
}

var data = `
enable_ssl: True
handle_cookie: True

host:
  self: mirror.loerfy.now.sh
  proxy: s2-us2.startpage.com

replaced_urls:
  - old: www.startpage.com
    new: s2-us2.startpage.com
  - old: s2-us2.startpage.com
    new: mirror.loerfy.now.sh
`

func loadConfig() {
	Config = new(Yaml)
	err := yaml.Unmarshal([]byte(data), Config)
	checkErr(err)
	if Config.EnableSSL {
		protocal = "https://"
	} else {
		protocal = "http://"
	}
}

func configStruct(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintf(w, "<h3><a href='/bye/bye'>Leave</a>")
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
func hasGziped(coding string) bool {
	return strings.HasPrefix(coding, "gz")
}

func isTextType(typeName string) bool {
	return strings.HasPrefix(typeName, "text") ||
		strings.HasPrefix(typeName, "appli")
}
func replaceText(text []byte) []byte {
	for _, url := range Config.ReplacedURLs {
		text = bytes.ReplaceAll(text,
			[]byte(url.Old), []byte(url.New))
	}
	return text
}

func replaceRedirect(header http.Header) string {
	domain := regexp.MustCompile(`://(.*?)/`)
	location := header.Get("Location")
	host := domain.FindStringSubmatch(location)[1]
	if host != Config.Host.Self {
		return strings.ReplaceAll(location, host, Config.Host.Self)
	}
	return location
}

func removeCookie(cookie string) string {
	domain := regexp.MustCompile(`(domain=.*?;)`)
	newCookit := domain.ReplaceAllLiteralString(cookie, "")
	return newCookit
}

func rewriteBody(resp *http.Response) (err error) {
	if nil != resp {
		defer resp.Body.Close()
	}
	checkErr(err)

	var content []byte
	cType := resp.Header.Get("Content-Type")
	cEncoding := resp.Header.Get("Content-Encoding")
	StatusCode := resp.StatusCode
	cookie := resp.Header.Get("Set-Cookie")

	if hasGziped(cEncoding) {
		resp.Header.Del("Content-Encoding")
		reader, _ := gzip.NewReader(resp.Body)
		content, err = ioutil.ReadAll(reader)
	} else {
		content, err = ioutil.ReadAll(resp.Body)
	}
	checkErr(err)
	if isTextType(cType) {
		content = replaceText(content)
	}
	if StatusCode == 302 || StatusCode == 301 {
		resp.Header.Set("Location", replaceRedirect(resp.Header))
	}
	if cookie != "" {
		resp.Header.Set("Set-Cookie", removeCookie(cookie))
	}

	resp.Body = ioutil.NopCloser(bytes.NewReader(content))
	resp.ContentLength = int64(len(content))
	resp.Header.Set("Content-Length", strconv.Itoa(len(content)))

	return nil
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if !initial {
		loadConfig()
		initial = true
	}
	rpURL, err := url.Parse(protocal + Config.Host.Proxy)
	checkErr(err)
	proxy := httputil.NewSingleHostReverseProxy(rpURL)
	proxy.ModifyResponse = rewriteBody
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.Host = Config.Host.Proxy
	}
	proxy.ServeHTTP(w, r)
}

// func main() {
// 	http.HandleFunc("/", Handler)
// 	// http.ListenAndServe(":3000", nil)
// 	http.ListenAndServeTLS(":3000", "/home/wincer/.local/share/mkcert/rootCA.pem", "/home/wincer/.local/share/mkcert/rootCA-key.pem", nil)
// 	log.Println("Listening in :3000")
// }
