package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/watch"
	"net/http"
	"sync"
	"time"
	"unicode"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	u "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"

	"github.com/aoxn/meridian/internal/server/service"
)

func readGetOption() *metav1.GetOptions {
	return &metav1.GetOptions{}
}

func readListOption() *metav1.ListOptions {
	return &metav1.ListOptions{}
}

func readDeleteOption() *metav1.DeleteOptions {
	return &metav1.DeleteOptions{}
}

func readCreateOption() *metav1.CreateOptions {
	return &metav1.CreateOptions{}
}

func readUpdateOption() *metav1.UpdateOptions {
	return &metav1.UpdateOptions{}
}

func NewHandler(grp service.Grouped, schem *runtime.Scheme) Handler {
	codec := serializer.NewCodecFactory(schem)
	return Handler{
		gvr:    grp,
		schema: schem,
		codec:  &codec,
		mu:     &sync.RWMutex{},
	}
}

type Handler struct {
	mu     *sync.RWMutex
	gvr    service.Grouped
	schema *runtime.Scheme
	codec  *serializer.CodecFactory
}

func (h *Handler) Routes() map[string]map[string]HandlerFunc {
	path := []string{
		"/{apiPrefix}/{version}/{resource}",
		"/{apiPrefix}/{version}/{resource}/{name}",
		"/{apiPrefix}/{version}/{resource}/{name}/{subresource}",
		"/{apiPrefix}/{version}/namespaces/{namespace}/{resource}",
		"/{apiPrefix}/{version}/namespaces/{namespace}/{resource}/{name}",
		"/{apiPrefix}/{version}/namespaces/{namespace}/{resource}/{name}/{subresource}",
		"/{apiPrefix}/{group}/{version}/{resource}",
		"/{apiPrefix}/{group}/{version}/{resource}/{name}",
		"/{apiPrefix}/{group}/{version}/{resource}/{name}/{subresource}",
		"/{apiPrefix}/{group}/{version}/namespaces/{namespace}/{resource}",
		"/{apiPrefix}/{group}/{version}/namespaces/{namespace}/{resource}/{name}",
		"/{apiPrefix}/{group}/{version}/namespaces/{namespace}/{resource}/{name}/{subresource}",
	}
	var (
		get  = make(map[string]HandlerFunc)
		put  = make(map[string]HandlerFunc)
		post = make(map[string]HandlerFunc)
		del  = make(map[string]HandlerFunc)
	)
	for _, v := range path {
		get[v] = h.List
		put[v] = h.Update
		post[v] = h.Create
		del[v] = h.Delete
	}
	routes := map[string]map[string]HandlerFunc{
		"GET":    get,
		"PUT":    put,
		"POST":   post,
		"DELETE": del,
	}
	return routes
}

func (h *Handler) findService(gv *schema.GroupVersionResource) service.Standard {
	return h.gvr.Service(gv)
}

func (h *Handler) List(
	ctx context.Context,
	write http.ResponseWriter,
	req *http.Request,
) {
	gvk := readResource(req)
	klog.V(5).Infof("List resource [%s] %v, %s", gvk.Verb, req.URL, gvk.GVR().String())
	storage := h.findService(gvk.GVR())
	if storage == nil {
		WriteErr(write, errors.NewBadRequest("Unkonwn storage"))
		return
	}
	switch gvk.Name {
	case "":
		option := readListOption()
		if option.Watch {
			item := storage.New(gvk.GVR())
			watcher, ok := storage.(service.Watcher)
			if !ok {
				WriteErr(write, errors.NewBadRequest("unimplemented method"))
				return
			}
			w, err := watcher.Watch(ctx, item, option)
			if err != nil {
				WriteErr(write, errors.NewInternalError(err))
				return
			}
			var serverShuttingDownCh <-chan struct{}
			server := &WatchServer{
				Watching: w,

				MediaType: "application/json",
				//Framer:          framer,
				//Encoder:         encoder,
				//EmbeddedEncoder: embeddedEncoder,
				ServerShuttingDownCh: serverShuttingDownCh,
			}
			server.HandleHTTP(ctx, write, req)
			return
		}

		lister, ok := storage.(service.Lister)
		if !ok {
			WriteErr(write, errors.NewBadRequest("unimplemented method"))
			return
		}
		items := storage.NewList(gvk.GVR())
		klog.V(5).Infof("build item: [%s]", gvk.GVR())
		_, err := lister.List(ctx, items, option)
		if err != nil {
			WriteErr(write, errors.NewInternalError(err))
			return
		}
		WriteJson(write, items, http.StatusOK)
	default:
		getter, ok := storage.(service.Getter)
		if !ok {
			WriteErr(write, errors.NewInternalError(fmt.Errorf("unimplement")))
			return
		}
		option := readGetOption()
		item := storage.New(gvk.GVR())
		metav, err := meta.Accessor(item)
		if err != nil {
			WriteErr(write, errors.NewInternalError(fmt.Errorf("unkonwn object:%s", err.Error())))
			return
		}
		metav.SetName(gvk.Name)
		metav.SetNamespace(gvk.Namespace)
		_, err = getter.Get(ctx, item, option)
		if err != nil {
			WriteErr(write, errors.NewInternalError(err))
			return
		}
		WriteJson(write, item, http.StatusOK)
	}
}

func toItems(k *u.UnstructuredList) string {
	o, err := k.MarshalJSON()
	if err != nil {
		return fmt.Sprintf("can not unmarshal unstructed list: %s", err.Error())
	}
	return string(o)
}

func (h *Handler) Create(
	ctx context.Context,
	write http.ResponseWriter,
	req *http.Request,
) {

	var (
		err    error
		target runtime.Object
	)
	gvk := readResource(req)
	storage := h.findService(gvk.GVR())
	if storage == nil {
		WriteErr(write, errors.NewBadRequest("unknown storage"))
		return
	}

	target = storage.New(gvk.GVR())
	err = h.decodeBody(req.Body, target)
	if err != nil {
		WriteErr(write, errors.NewBadRequest(err.Error()))
		return
	}

	creater, ok := storage.(service.Creater)
	if !ok {
		WriteErr(write, errors.NewBadRequest("unimplemented method"))
		return
	}
	option := readCreateOption()
	items, err := creater.Create(ctx, target, option)
	if err != nil {
		WriteErr(write, errors.NewInternalError(err))
		return
	}
	WriteJson(write, items, http.StatusOK)
}

func (h *Handler) Delete(
	ctx context.Context,
	write http.ResponseWriter,
	req *http.Request) {
	var (
		err    error
		target runtime.Object
	)
	gvk := readResource(req)
	storage := h.findService(gvk.GVR())
	if storage == nil {
		WriteErr(write, errors.NewBadRequest("Unknown storage"))
		return
	}

	target = storage.New(gvk.GVR())
	err = h.decodeBody(req.Body, target)
	if err != nil {
		WriteErr(write, errors.NewBadRequest(err.Error()))
		return
	}
	ob, err := meta.Accessor(target)
	if err != nil {
		WriteErr(write, errors.NewInternalError(err))
		return
	}
	if ob.GetName() == "" {
		ob.SetName(gvk.Name)
	}
	deleter, ok := storage.(service.GracefulDeleter)
	if !ok {
		WriteErr(write, errors.NewBadRequest("unimplemented method"))
		return
	}
	option := readDeleteOption()
	items, err := deleter.Delete(ctx, target, option)
	if err != nil {
		WriteErr(write, errors.NewInternalError(err))
		return
	}
	WriteJson(write, items, http.StatusOK)
}

func (h *Handler) Update(
	ctx context.Context,
	write http.ResponseWriter,
	req *http.Request) {
	var (
		err    error
		target runtime.Object
	)
	gvk := readResource(req)
	storage := h.findService(gvk.GVR())
	if storage == nil {
		WriteErr(write, errors.NewBadRequest("Unkonwn storage"))
		return
	}

	target = storage.New(gvk.GVR())
	err = h.decodeBody(req.Body, target)
	if err != nil {
		WriteErr(write, errors.NewBadRequest(err.Error()))
		return
	}

	updater, ok := storage.(service.Updater)
	if !ok {
		WriteErr(write, errors.NewBadRequest("unimplemented method"))
		return
	}

	option := readUpdateOption()
	items, err := updater.Update(ctx, target, option)
	if err != nil {
		WriteErr(write, errors.NewInternalError(err))
		return
	}
	WriteJson(write, items, http.StatusOK)
}

func (h *Handler) decodeBody(body io.ReadCloser, into runtime.Object) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	decoder := h.codec.UniversalDecoder()
	_, _, err = decoder.Decode(data, nil, into)
	return err
}

func gvk(r string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "meridian.meridian.io", Version: "v1", Kind: toUpper(r)}
}

func toUpper(r string) string {
	if r == "" {
		return ""
	}
	k := []rune(r)
	k[0] = unicode.ToUpper(k[0])
	return string(k)
}

func WriteJsonErr(w http.ResponseWriter, msg string, code int) int {
	return WriteJson(w, Status{Message: msg}, code)
}

func WriteJson(w http.ResponseWriter, v interface{}, code int) int {
	var text string

	switch v.(type) {
	case error:
		text = v.(error).Error()
	case string:
		text = v.(string)
	default:
		resp, err := json.Marshal(v)
		if err != nil {
			text = err.Error()
			break
		}
		text = string(resp)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, err := io.Copy(w, bytes.NewBuffer([]byte(text)))
	if err != nil {
		klog.Errorf("copy response: %s", err.Error())
	}
	return code
}

func WriteErr(w http.ResponseWriter, v *errors.StatusError) int {
	text, err := json.Marshal(v)
	if err != nil {
		text = []byte(err.Error())
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(int(v.Status().Code))
	_, err = io.Copy(w, bytes.NewBuffer(text))
	if err != nil {
		klog.Errorf("copy response: %s", err.Error())
		return int(v.Status().Code)
	}
	return int(v.Status().Code)
}

type Status struct {
	HttpCode int    `json:"code,omitempty" protobuf:"bytes,1,opt,name=code"`
	Status   string `json:"status,omitempty" protobuf:"bytes,2,opt,name=status"`
	Message  string `json:"message,omitempty" protobuf:"bytes,3,opt,name=message"`
}

// WatchServer serves a watch.Interface over a websocket or vanilla HTTP.
type WatchServer struct {
	Watching watch.Interface
	// the media type this watch is being served with
	MediaType string
	// used to frame the watch stream
	Framer runtime.Framer
	// used to encode the watch stream event itself
	Encoder runtime.Encoder
	// used to encode the nested object in the watch stream
	EmbeddedEncoder      runtime.Encoder
	ServerShuttingDownCh <-chan struct{}
}

// HandleHTTP serves a series of encoded events via HTTP with Transfer-Encoding: chunked.
// or over a websocket connection.
func (s *WatchServer) HandleHTTP(ctx context.Context, w http.ResponseWriter, req *http.Request) {

	flusher, ok := w.(http.Flusher)
	if !ok {
		err := fmt.Errorf("unable to start watch - can't get http.Flusher: %#v", w)
		WriteErr(w, errors.NewInternalError(err))
		return
	}

	framer := s.Framer.NewFrameWriter(w)
	if framer == nil {
		// programmer error
		err := fmt.Errorf("no stream framing support is available for media type %q", s.MediaType)
		WriteErr(w, errors.NewBadRequest(err.Error()))
		return
	}

	// ensure the connection times out
	timeoutCh, cleanup := context.WithTimeout(ctx, 30*time.Minute)
	defer cleanup()

	// begin the stream
	w.Header().Set("Content-Type", s.MediaType)
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	//watchEncoder := newWatchEncoder(req.Context(), kind, s.EmbeddedEncoder, s.Encoder, framer)

	//ch := s.Watching.ResultChan()
	done := req.Context().Done()

	for {
		select {
		case <-s.ServerShuttingDownCh:
			// the server has signaled that it is shutting down (not accepting
			// any new request), all active watch request(s) should return
			// immediately here. The WithWatchTerminationDuringShutdown server
			// filter will ensure that the response to the client is rate
			// limited in order to avoid any thundering herd issue when the
			// client(s) try to reestablish the WATCH on the other
			// available apiserver instance(s).
			return
		case <-done:
			return
		case <-timeoutCh.Done():
			return
			//case event, ok := <-ch:
			//	if !ok {
			//		// End of results.
			//		return
			//	}
			//	if err := watchEncoder.Encode(event); err != nil {
			//		utilruntime.HandleError(err)
			//		// client disconnect.
			//		return
			//	}
			//
			//	if len(ch) == 0 {
			//		flusher.Flush()
			//	}
		}
	}
}
