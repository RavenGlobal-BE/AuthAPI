package Logging

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestLog(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Log("test message", Info)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "INFO") {
		t.Error("Expected INFO label")
	}
}
