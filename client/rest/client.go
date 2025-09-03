/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rest

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

const (
	// Environment variables: Note that the duration should be long enough that the backoff
	// persists for some reasonable time (i.e. 120 seconds).  The typical base might be "1".
	envBackoffBase     = "KUBE_CLIENT_BACKOFF_BASE"
	envBackoffDuration = "KUBE_CLIENT_BACKOFF_DURATION"
)

// Interface captures the set of operations for generically interacting with Kubernetes REST apis.
type Interface interface {
	Verb(ctx context.Context, verb string) *Request
	Post(ctx context.Context) *Request
	Put(ctx context.Context) *Request
	Patch(ctx context.Context, pt string) *Request
	Get(ctx context.Context) *Request
	Delete(ctx context.Context) *Request
	APIVersion(ctx context.Context) string
}

// RESTClient imposes common Kubernetes API conventions on a set of resource paths.
// The baseURL is expected to point to an HTTP or HTTPS path that is the parent
// of one or more resources.  The server should return a decodable API resource
// object, or an api.Status object which contains information about the reason for
// any failure.
//
// Most consumers should use client.New() to get a Kubernetes API client.
type RESTClient struct {
	ContentType string
	// base is the root URL for all invocations of the client
	base *url.URL
	// versionedAPIPath is a path segment connecting the base URL to the resource root
	versionedAPIPath string

	// Set specific behavior of the client.  If not set http.DefaultClient will be used.
	Client *http.Client
}

// NewRESTClient creates a new RESTClient. This client performs generic REST functions
// such as Get, Put, Post, and Delete on specified paths.  Codec controls encoding and
// decoding of responses from the server.
func NewRESTClient(
	baseURL *url.URL,
	contentType string,
	maxQPS float32, maxBurst int,
	//rateLimiter flowcontrol.RateLimiter,
	client *http.Client,
) (*RESTClient, error) {

	base := *baseURL
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	base.RawQuery = ""
	base.Fragment = ""

	return &RESTClient{
		ContentType:      contentType,
		base:             &base,
		versionedAPIPath: "",
		//Throttle:         throttle,
		Client: client,
	}, nil
}

// Verb begins a request with a verb (GET, POST, PUT, DELETE).
//
// Example usage of RESTClient's request building interface:
// c, err := NewRESTClient(...)
// if err != nil { ... }
// resp, err := c.Verb("GET").
//
//	Path("pods").
//	SelectorParam("labels", "area=staging").
//	Timeout(10*time.Second).
//	Do()
//
// if err != nil { ... }
// list, ok := resp.(*api.PodList)
func (c *RESTClient) Verb(ctx context.Context, verb string) *Request {

	return NewRequest(
		ctx,
		c.Client,
		verb, c.base,
		c.versionedAPIPath,
		c.ContentType,
	)
}

// Post begins a POST request. Short for c.Verb("POST").
func (c *RESTClient) Post(ctx context.Context) *Request {
	return c.Verb(ctx, "POST")
}

// Put begins a PUT request. Short for c.Verb("PUT").
func (c *RESTClient) Put(ctx context.Context) *Request {
	return c.Verb(ctx, "PUT")
}

// Patch begins a PATCH request. Short for c.Verb("Patch").
func (c *RESTClient) Patch(ctx context.Context, pt string) *Request {
	return c.Verb(ctx, "PATCH").SetHeader("Content-Type", string(pt))
}

// Get begins a GET request. Short for c.Verb("GET").
func (c *RESTClient) Get(ctx context.Context) *Request {
	return c.Verb(ctx, "GET")
}

// Delete begins a DELETE request. Short for c.Verb("DELETE").
func (c *RESTClient) Delete(ctx context.Context) *Request {
	return c.Verb(ctx, "DELETE")
}

// APIVersion returns the APIVersion this RESTClient is expected to use.
func (c *RESTClient) APIVersion(ctx context.Context) string {
	return "v1"
}
