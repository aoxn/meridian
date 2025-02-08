package server

import (
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ResourceInfo struct {
	// Path is the URL path of the request
	Path string
	// Verb is the kube verb associated with the request for API requests, not the http verb.  This includes things like list and watch.
	// for non-resource requests, this is the lowercase http verb
	Verb string

	APIPrefix  string
	APIGroup   string
	APIVersion string
	Namespace  string
	// Resource is the name of the resource being requested.  This is not the kind.  For example: pods
	Resource string
	// Subresource is the name of the subresource being requested.  This is a different resource, scoped to the parent resource, but it may have a different kind.
	// For instance, /pods has the resource "pods" and the kind "Pod", while /pods/foo/status has the resource "pods", the sub resource "status", and the kind "Pod"
	// (because status operates on pods). The binding resource for a pod though may be /pods/foo/binding, which has resource "pods", subresource "binding", and kind "Binding".
	Subresource string
	// Name is empty for some verbs, but if the request directly indicates a name (not in body content) then this field is filled in.
	Name string
	// Parts are the path parts for the request, always starting with /{resource}/{name}
	Parts []string
	//UALimitVerb is used for user-agent limiter
	UALimitVerb string
}

func (r *ResourceInfo) GVR() *schema.GroupVersionResource {
	return &schema.GroupVersionResource{
		Group:    r.APIGroup,
		Version:  r.APIVersion,
		Resource: r.Resource,
	}
}

func (r *ResourceInfo) APIGroupVersion() *schema.GroupVersion {
	return &schema.GroupVersion{
		Group:   r.APIGroup,
		Version: r.APIVersion,
	}
}

// readResource returns the information from the http request.
// If error is not nil, RequestInfo holds the information as best it is known before the failure
// It handles both resource and non-resource requests and fills in all the pertinent information for each.
// Valid Inputs:
// Resource paths
// /apis/{api-group}/{version}/namespaces
// /api/{version}/namespaces
// /api/{version}/namespaces/{namespace}
// /api/{version}/namespaces/{namespace}/{resource}
// /api/{version}/namespaces/{namespace}/{resource}/{resourceName}
// /api/{version}/{resource}
// /api/{version}/{resource}/{resourceName}
//
// Special verbs without subresources:
// /api/{version}/proxy/{resource}/{resourceName}
// /api/{version}/proxy/namespaces/{namespace}/{resource}/{resourceName}
//
// Special verbs with subresources:
// /api/{version}/watch/{resource}
// /api/{version}/watch/namespaces/{namespace}/{resource}
//
// NonResource paths
// /apis/{api-group}/{version}
// /apis/{api-group}
// /apis
// /api/{version}
// /api
// /healthz
// /
// TODO write an integration test against the swagger doc to test the RequestInfo and match up behavior to responses
func readResource(req *http.Request) *ResourceInfo {
	info := ResourceInfo{
		Path: req.URL.Path,
		Verb: strings.ToLower(req.Method),
	}

	parts := splitPath(req.URL.Path)
	if len(parts) < 3 {
		// return a non-resource request
		return &info
	}

	if !prefix.Has(parts[0]) {
		// return a non-resource request
		return &info
	}
	info.APIPrefix = parts[0]
	parts = parts[1:]

	if !grouplessPrefix.Has(info.APIPrefix) {
		// one part (APIPrefix) has already been consumed,
		// so this is actually "do we have four parts?"
		if len(parts) < 3 {
			// return a non-resource request
			return &info
		}
		info.APIGroup = parts[0]
		parts = parts[1:]
	}

	info.APIVersion = parts[0]
	parts = parts[1:]

	// URL forms: /namespaces/{namespace}/{kind}/*, where parts are adjusted to be relative to kind
	if parts[0] == "namespaces" {
		if len(parts) > 1 {
			info.Namespace = parts[1]

			// if there is another step after the namespace name and it is not a known namespace subresource
			// move parts to include it as a resource in its own right
			if len(parts) > 2 && !namespaceSubresources.Has(parts[2]) {
				parts = parts[2:]
			}
		}
	} else {
		info.Namespace = metav1.NamespaceNone
	}

	// parsing successful, so we now know the proper value for .Parts
	info.Parts = parts

	// parts look like: resource/resourceName/subresource/other/stuff/we/don't/interpret
	switch {
	case len(info.Parts) >= 3 && !specialNoSubresource.Has(info.Verb):
		info.Subresource = info.Parts[2]
		fallthrough
	case len(info.Parts) >= 2:
		info.Name = info.Parts[1]
		fallthrough
	case len(info.Parts) >= 1:
		info.Resource = info.Parts[0]
	}

	return &info
}

var (
	prefix = sets.NewString("api", "apis")

	grouplessPrefix = sets.NewString("api")

	// specialVerbs contains just strings which are used in REST paths for
	// special actions that don't fall under the normal CRUDdy GET/POST/PUT/DELETE
	// actions on REST objects.
	specialVerbs = sets.NewString("proxy", "watch")

	// specialNoSubresource contains root verbs which do not allow subresources
	specialNoSubresource = sets.NewString("proxy")

	// namespaceSubresources contains subresources of namespace
	// this list allows the parser to distinguish between a namespace subresource, and a namespaced resource
	namespaceSubresources = sets.NewString("status", "finalize")
)

type requestInfoKeyType int

// requestInfoKey is the RequestInfo key for the context. It's of private type here. Because
// keys are interfaces and interfaces are equal when the type and the value is equal, this
// does not conflict with the keys defined in pkg/api.
const requestInfoKey requestInfoKeyType = iota

// splitPath returns the segments for a URL path.
func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}
