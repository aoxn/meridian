package rest

import (
	"bytes"
	"errors"
	"fmt"
	gerrors "github.com/pkg/errors"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"encoding/json"
	"os"

	"k8s.io/apimachinery/pkg/util/wait"
	"path"
)

// HTTPClient is an interface for testing a request object.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewRequest(
	client HTTPClient,
	verb string,
	baseURL *url.URL,
	versionedAPIPath string,
	contentType string,
) *Request {

	pathPrefix := "/"
	if baseURL != nil {
		pathPrefix = path.Join(pathPrefix, baseURL.Path)
	}
	r := &Request{
		client:     client,
		verb:       verb,
		baseURL:    baseURL,
		pathPrefix: path.Join(pathPrefix, versionedAPIPath),
		//timeout:     timeout,
	}
	r.SetHeader("Accept", contentType+", */*")
	return r
}

// Request allows for building up a request to a server in a chained fashion.
// Any errors are stored until the end of your call, so you only have to
// check once.
type Request struct {
	// required
	client HTTPClient
	verb   string

	baseURL *url.URL

	// generic components accessible via method setters
	pathPrefix string
	params     url.Values
	headers    http.Header

	resource string

	//id of the resource
	resourceName string

	subresource string
	timeout     time.Duration

	// output
	err  error
	body io.Reader
}

/*
	api/v1/nodes
*/

func (req *Request) PathPrefix(prefix string) *Request {
	req.pathPrefix = prefix
	return req
}

func (req *Request) Resource(resource string) *Request {
	req.resource = resource
	return req
}

func (req *Request) ResourceName(name string) *Request {
	req.resourceName = name
	return req
}

func (req *Request) SubResource(sub string) *Request {
	req.subresource = sub
	return req
}

func (req *Request) SetHeader(key string, values ...string) *Request {
	if req.headers == nil {
		req.headers = http.Header{}
	}
	req.headers.Del(key)
	for _, value := range values {
		req.headers.Add(key, value)
	}
	return req
}

// Body makes the request use obj as the body. Optional.
// If obj is a string, try to read a file of that name.
// If obj is a []byte, send it directly.
// If obj is an io.Reader, use it directly.
// If obj is a runtime.Object, marshal it correctly, and set Content-Type header.
// If obj is a runtime.Object and nil, do nothing.
// Otherwise, set an error.
func (req *Request) Body(obj interface{}) *Request {
	if req.err != nil {
		klog.Errorf("request error: %s", req.err.Error())
		return req
	}
	switch t := obj.(type) {
	case string:
		data, err := ioutil.ReadFile(t)
		if err != nil {
			req.err = err
			return req
		}

		req.body = bytes.NewReader(data)
	case []byte:

		req.body = bytes.NewReader(t)
	case io.Reader:
		req.body = t
	default:
		// callers may pass typed interface pointers, therefore we must check nil with reflection
		if reflect.ValueOf(t).IsNil() {
			return req
		}

		data, err := json.Marshal(obj)
		if err != nil {
			req.err = err
			return req
		}
		req.body = bytes.NewReader(data)
		req.SetHeader("Content-Type", "application/json")
	}
	return req
}

func (req *Request) Url() (string, error) {

	if req.resource == "" {
		return "", errors.New("the resource you want to visit must not be nil")
	}

	endpoint := os.Getenv("ENDPOINT")
	if endpoint == "" {
		endpoint = fmt.Sprintf("%s://%s", req.baseURL.Scheme, req.baseURL.Host)
	}
	r := path.Join(
		req.pathPrefix, req.resource, req.resourceName,
	)
	return fmt.Sprintf("%s%s", endpoint, r), nil
}

func (req *Request) Do(api interface{}) error {
	var (
		res  = ""
		last error
	)
	u, err := req.Url()
	if err != nil {
		return gerrors.Wrapf(err, "build url")
	}
	klog.V(5).Infof("[%s]send request: %s", req.verb, u)
	err = wait.ExponentialBackoff(
		wait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   1,
			Steps:    1,
		},
		func() (bool, error) {
			res, last = req.send()

			if shouldRetry(last) {
				return false, nil
			}
			if last != nil {
				return true, last
			}
			return true, nil
		},
	)
	if err != nil {
		if !strings.Contains(err.Error(), timedOut) {
			return err
		}
		return errors.New(fmt.Sprintf("do request[%s=%s] error(timed out): lastError=%v ", req.verb, req.resource, last))
	}
	klog.V(8).Infof("debug request result: %s", res)
	if len(res) <= 0 {
		return nil
	}
	return req.Decode(res, api)
}

func (req *Request) DirectDo() error {
	var (
		last error
	)
	u, err := req.Url()
	if err != nil {
		return gerrors.Wrapf(err, "build url")
	}
	klog.V(5).Infof("[%s]send request: %s", req.verb, u)
	err = wait.ExponentialBackoff(
		wait.Backoff{
			Duration: 500 * time.Millisecond,
			Factor:   1,
			Steps:    1,
		},
		func() (bool, error) {
			_, last = req.send()

			if shouldRetry(last) {
				return false, nil
			}
			if last != nil {
				return true, last
			}
			return true, nil
		},
	)
	if err != nil {
		if !strings.Contains(err.Error(), timedOut) {
			return err
		}
		return errors.New(fmt.Sprintf("do request[%s=%s] error(timed out): lastError=%v ", req.verb, req.resource, last))
	}
	return err
}

var timedOut = "timed out"

func (req *Request) Decode(data string, api interface{}) error {
	if data == "" {
		//url, _ := req.Url()
		//klog.Infof("client: empty body data received, url %s", url)
		return nil
	}
	return json.Unmarshal([]byte(data), api)
}

func (req *Request) send() (string, error) {
	url, err := req.Url()
	if err != nil {
		return "", err
	}

	klog.V(8).Infof("send request to %s", url)
	// TODO: body should be reset on retry
	requ, err := http.NewRequest(req.verb, url, req.body)

	if err != nil {
		return "", err
	}
	if req.client == nil {
		req.client = &http.Client{}
	}
	resp, err := req.client.Do(requ)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("request code: %d, %s", resp.StatusCode, err.Error())
		}
		return "", fmt.Errorf("request: code: %d, data[%s]", resp.StatusCode, data)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type TimeoutError interface {
	error
	Timeout() bool // Is the error a timeout?
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(TimeoutError)
	if ok {
		return true
	}

	switch err {
	case io.ErrUnexpectedEOF, io.EOF:
		return true
	}
	switch e := err.(type) {
	case *net.DNSError:
		return true
	case *net.OpError:
		switch e.Op {
		case "read", "write":
			return true
		}
	case *url.Error:
		// url.Error can be returned either by net/url if a URL cannot be
		// parsed, or by net/http if the response is closed before the headers
		// are received or parsed correctly. In that later case, e.Op is set to
		// the HTTP method name with the first letter uppercased. We don't want
		// to retry on POST operations, since those are not idempotent, all the
		// other ones should be safe to retry.
		switch e.Op {
		case "Get", "Put", "Delete", "Head":
			return shouldRetry(e.Err)
		default:
			return false
		}
	}
	return false
}

type ResultList struct {
	result []string
}
