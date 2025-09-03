package watch

import (
	"fmt"
	"testing"
)

func TestWatch(t *testing.T) {
	watcher := NewStreamWatcher(&emptySource{
		data: map[string]string{"a": "", "b": "", "c": ""},
	})
	for event := range watcher.ResultChan() {
		t.Logf("receieve %s: %s", event.Type, event.Object)
		if event.Type == Error {
			t.Log("finished")
			break
		}
	}
	panicWatcher := NewStreamWatcher(&emptySource{
		panic: true,
	})
	for event := range panicWatcher.ResultChan() {
		t.Logf("receieve %s: %s", event.Type, event.Object)
	}
}

type emptySource struct {
	data  map[string]string
	panic bool
}

func (d *emptySource) Decode() (object any, err error) {
	if d.panic {
		panic("panic test")
	}
	for k, v := range d.data {
		if v != "send" {
			d.data[k] = "send"
			return k, nil
		}
	}
	return "", fmt.Errorf("empty")
}

func (d *emptySource) Close() {
}
