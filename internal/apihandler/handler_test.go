package apihandler

import (
	"context"
	"testing"
)

func TestAPIHandler(t *testing.T) {
	err := RunDaemonAPI(context.TODO())
	if err != nil {
		t.Fatalf("failed to run daemon api handler: %v", err)
	}
}
