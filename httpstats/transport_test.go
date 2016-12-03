package httpstats

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/stats"
)

func TestTransport(t *testing.T) {
	newRequest := func(method string, path string, body io.Reader) *http.Request {
		req, _ := http.NewRequest(method, path, body)
		return req
	}

	for _, transport := range []http.RoundTripper{
		nil,
		&http.Transport{},
		http.DefaultTransport,
		http.DefaultClient.Transport,
	} {
		t.Run("", func(t *testing.T) {
			for _, req := range []*http.Request{
				newRequest("GET", "/", nil),
				newRequest("POST", "/", strings.NewReader("Hi")),
			} {
				t.Run("", func(t *testing.T) {
					engine := stats.NewDefaultEngine()
					defer engine.Close()

					server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
						ioutil.ReadAll(req.Body)
						res.Write([]byte("Hello World!"))
					}))
					defer server.Close()

					httpc := &http.Client{
						Transport: NewTransport(engine, transport),
					}

					req.URL.Scheme = "http"
					req.URL.Host = server.URL[7:]

					res, err := httpc.Do(req)
					if err != nil {
						t.Error(err)
						return
					}
					ioutil.ReadAll(res.Body)
					res.Body.Close()

					// Let the engine process the metrics.
					time.Sleep(10 * time.Millisecond)

					metrics, _ := engine.State(0)

					if len(metrics) == 0 {
						t.Error("no metrics reported by http handler")
					}

					for _, m := range metrics {
						for _, tag := range m.Tags {
							if tag.Name == "bucket" {
								switch tag.Value {
								case "2xx", "":
								default:
									t.Errorf("invalid bucket in metric event tags: %#v\n%#v", tag, m)
								}
							}
						}
					}
				})
			}
		})
	}
}

func TestTransportError(t *testing.T) {
	engine := stats.NewDefaultEngine()
	defer engine.Close()

	server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		conn, _, _ := res.(http.Hijacker).Hijack()
		conn.Close()
	}))
	defer server.Close()

	httpc := &http.Client{
		Transport: NewTransport(engine, &http.Transport{}),
	}

	if _, err := httpc.Post(server.URL, "text/plain", strings.NewReader("Hi")); err == nil {
		t.Error("no error was reported by the http client")
	}

	// Let the engine process the metrics.
	time.Sleep(10 * time.Millisecond)

	metrics, _ := engine.State(0)

	if len(metrics) == 0 {
		t.Error("no metrics reported by hijacked http handler")
	}
}
