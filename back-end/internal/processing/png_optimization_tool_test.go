package processing

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPNGNativeToolFailuresRetainOriginal(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "mock.go")
	writePNGTestFile(t, source, []byte(`package main
import("fmt";"os";"strings")
func main(){
 mode:=os.Getenv("FABRICUM_PNG_TEST_TOOL_MODE")
 if len(os.Args)==2 && os.Args[1]=="--version" {
  if mode=="wrong-version" {fmt.Println("oxipng 1.0.0")} else {fmt.Println("oxipng 10.2.1")};return
 }
 if mode=="failure" {fmt.Fprintln(os.Stderr,"fixture optimizer failure");os.Exit(7)}
 if mode=="missing-output" {return}
 for i,arg:=range os.Args {if arg=="--out" {os.WriteFile(os.Args[i+1],[]byte(strings.Repeat("invalid",10)),0600)}}
}`))
	command := exec.Command("go", "build", "-o", nativeToolPath(directory, "oxipng"), source)
	if data, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build mock optimizer: %v: %s", err, data)
	}
	input := pngOptimizationFixtures(t)["rgb"]
	for _, sample := range []struct{ mode, diagnostic string }{
		{"wrong-version", "requires oxipng"},
		{"failure", "fixture optimizer failure"},
		{"missing-output", "read oxipng output"},
		{"invalid-output", "invalid oxipng output"},
	} {
		t.Run(sample.mode, func(t *testing.T) {
			t.Setenv("FABRICUM_PNG_TEST_TOOL_MODE", sample.mode)
			path := filepath.Join(t.TempDir(), "input.png")
			writePNGTestFile(t, path, input)
			result, err := OptimizePNG(context.Background(), PNGOptimizationRequest{Path: path, EncoderDirectory: directory})
			if err == nil || !strings.Contains(err.Error(), sample.diagnostic) || result.Changed {
				t.Fatalf("unexpected diagnostic: %+v, %v", result, err)
			}
			output, _ := os.ReadFile(path)
			if !bytes.Equal(input, output) {
				t.Fatal("native tool failure changed original")
			}
		})
	}
}
