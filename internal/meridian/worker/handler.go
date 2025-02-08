package worker

import (
	"context"
)

type Request struct {
	Key       string
	WorkerID  string
	QueueName string
	Param     map[string]interface{}
}

type Response struct {
}

type Handler interface {
	Handle(ctx context.Context, req *Request, rep *Response) error
}

type EventAwareHandler struct {
	freeze *Action
}
