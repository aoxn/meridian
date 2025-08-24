// From https://raw.githubusercontent.com/norouter/norouter/v0.6.5/pkg/agent/bicopy/bicopy.go
/*
   Copyright (C) NoRouter authors.

   Copyright (C) libnetwork authors.

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

package forward

import (
	"io"
	"k8s.io/klog/v2"
	"net"
	"sync"
)

// Bicopy is from https://github.com/rootless-containers/rootlesskit/blob/v0.10.1/pkg/port/builtin/parent/tcp/tcp.go#L73-L104
// (originally from libnetwork, Apache License 2.0).
func Bicopy(x, y net.Conn, quit <-chan struct{}) {
	var wg sync.WaitGroup
	broker := func(to, from net.Conn) {
		cnt, err := io.Copy(to, from)
		if err != nil {
			klog.Errorf("[%p]failed to call io.Copy:[%d] copied,  %s", &to, cnt, err.Error())
		}
		if conn, ok := from.(*net.TCPConn); ok {
			if err := conn.CloseRead(); err != nil {
				klog.Errorf("[%p]failed to call CloseRead: %s", &to, err.Error())
			}
		} else {
			klog.Infof("[%p]debug bicopy: no CloseRead", &to)
		}
		if conn, ok := to.(*net.TCPConn); ok {
			if err := conn.CloseWrite(); err != nil {
				klog.Errorf("[%p]failed to call CloseWrite: %s", &to, err.Error())
			}
		} else {
			klog.Infof("[%p]debug bicopy: no CloseWrite", &to)
		}

		klog.Infof("[%p]debug bicopy: finished, [%d] bytes copied", &to, cnt)
		wg.Done()
	}

	wg.Add(2)
	go broker(x, y)
	go broker(y, x)
	finish := make(chan struct{})
	go func() {
		wg.Wait()
		close(finish)
	}()

	select {
	case <-quit:
	case <-finish:
	}

	if err := x.Close(); err != nil {
		klog.Errorf("[%p]failed to call xCloser.Close: %s", &x, err.Error())
	}
	if err := y.Close(); err != nil {
		klog.Errorf("[%p]failed to call yCloser.Close: %s", &x, err.Error())
	}
	<-finish
	klog.Errorf("[%s,  %s,  %s,  %s]close stream finished", x.RemoteAddr(), x.LocalAddr(), y.LocalAddr(), y.RemoteAddr())
	// TODO: return copied bytes
}
