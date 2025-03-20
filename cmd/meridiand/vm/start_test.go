package vm

import "testing"

func TestStart(t *testing.T) {
	err := Start([]string{}, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("start ok")
}
