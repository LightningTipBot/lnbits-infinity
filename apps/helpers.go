package apps

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"reflect"
	"strings"

	"github.com/aarzilli/golua/lua"
)

var (
	float64type = reflect.TypeOf(0.0)
	booltype    = reflect.TypeOf(true)
)

func appidToURL(appid string) string {
	if url, err := base64.StdEncoding.DecodeString(appid); err == nil {
		return string(url)
	} else {
		log.Warn().Err(err).Str("appid", appid).Msg("got invalid app id")
		return ""
	}
}

func stacktrace(luaError *lua.LuaError) string {
	stack := luaError.StackTrace()
	message := []string{luaError.Error() + "\n"}
	for i := len(stack) - 1; i >= 0; i-- {
		entry := stack[i]
		message = append(message, entry.Name+" "+entry.ShortSource)
	}
	return strings.Join(message, "\n")
}

// convert struct to map[string]interface{}
func structToMap(v interface{}) map[string]interface{} {
	j, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(j, &m)
	return m
}

func urljoin(baseURL url.URL, elems ...string) *url.URL {
	for _, elem := range elems {
		if strings.HasPrefix(elem, "/") {
			baseURL.Path = elem
		} else if strings.HasPrefix(elem, "http") {
			newBaseURL, _ := url.Parse(elem)
			return newBaseURL
		} else if strings.HasSuffix(baseURL.Path, "/") {
			baseURL.Path = path.Join(baseURL.Path, elem)
		} else {
			spl := strings.Split(baseURL.Path, "/")
			pathWithoutLastPart := strings.Join(spl[0:len(spl)-1], "/")
			baseURL.Path = path.Join(pathWithoutLastPart, elem)
		}
	}

	return &baseURL
}

func serveFile(w http.ResponseWriter, r *http.Request, fileURL *url.URL) {
	(&httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL = fileURL
			r.Host = fileURL.Host
		},
		Transport: getStaticResourceTransporter{},
		ModifyResponse: func(w *http.Response) error {
			if w.StatusCode >= 400 {
				response, _ := ioutil.ReadAll(w.Body)
				w.Body.Close()
				w.Body = io.NopCloser(bytes.NewBuffer(
					[]byte(fmt.Sprintf("%s said: %s", fileURL, string(response))),
				))
			}
			return nil
		},
	}).ServeHTTP(w, r)
}

// this is an http.RoundTripper that only uses the URL and ignores the rest of the request
// used only to proxy the static files for the apps on serverFile above.
// this is needed because the default transporter was causing Caddy to return a 400
// for mysterious reasons that would be too painful and useless to investigate.
type getStaticResourceTransporter struct{}

func (_ getStaticResourceTransporter) RoundTrip(r *http.Request) (*http.Response, error) {
	return httpClient.Get(r.URL.String())
}
