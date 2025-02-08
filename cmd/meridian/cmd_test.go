package meridian

import (
	"fmt"
	"testing"
	"unicode"
)

func TestRunex(t *testing.T) {
	r := "abc"
	b := []rune(r)
	k := unicode.ToUpper([]rune(r)[0])
	b[0] = k
	fmt.Println(string(b))
}
