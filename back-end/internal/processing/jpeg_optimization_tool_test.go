package processing

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJPEGNativeToolFailuresRetainOriginal(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "mock.go")
	writePNGTestFile(t, source, []byte(`package main
import("fmt";"os";"path/filepath";"time")
func main(){
 mode:=os.Getenv("FABRICUM_JPEG_TEST_TOOL_MODE")
 if len(os.Args)==2 && os.Args[1]=="-version" {
  if mode=="wrong-version" {fmt.Println("libjpeg-turbo version 1.0.0")} else {fmt.Println("libjpeg-turbo version 3.1.3 (build fixture)")};return
 }
 if mode=="failure" {fmt.Fprintln(os.Stderr,"fixture optimizer failure");os.Exit(7)}
 if mode=="timeout" {time.Sleep(time.Minute)}
 if mode=="missing-output" {return}
 input,_:=os.ReadFile(os.Args[len(os.Args)-1])
 for i,arg:=range os.Args {if arg=="-outfile" {
  if filepath.Base(os.Args[i+1])=="candidate.jpg" {
   if mode=="invalid-output" {input=[]byte("corrupt")} else if mode=="changed-coefficients" {input,_=os.ReadFile(os.Getenv("FABRICUM_JPEG_TEST_REENCODED"))}
  }
  os.WriteFile(os.Args[i+1],input,0600)
 }}
}`))
	command := exec.Command("go", "build", "-o", nativeToolPath(directory, "jpegtran"), source)
	if data, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build mock: %v: %s", err, data)
	}
	input := jpegOptimizationFixture(t, false)
	decoded, _ := jpeg.Decode(bytes.NewReader(input))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, decoded, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	reencoded := filepath.Join(directory, "reencoded.jpg")
	writePNGTestFile(t, reencoded, encoded.Bytes())
	t.Setenv("FABRICUM_JPEG_TEST_REENCODED", reencoded)
	for _, sample := range []struct{ mode, diagnostic string }{
		{"wrong-version", "requires libjpeg-turbo"},
		{"failure", "fixture optimizer failure"},
		{"missing-output", "read jpegtran output"},
		{"invalid-output", "invalid jpegtran output"},
		{"changed-coefficients", "lossless DCT coefficient contract"},
		{"timeout", "canceled"},
	} {
		t.Run(sample.mode, func(t *testing.T) {
			t.Setenv("FABRICUM_JPEG_TEST_TOOL_MODE", sample.mode)
			path := filepath.Join(t.TempDir(), "input.jpg")
			writePNGTestFile(t, path, input)
			ctx := context.Background()
			if sample.mode == "timeout" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
			}
			result, err := OptimizeJPEG(ctx, JPEGOptimizationRequest{Path: path, EncoderDirectory: directory})
			if err == nil || !strings.Contains(err.Error(), sample.diagnostic) || result.Changed {
				t.Fatalf("result: %+v, %v", result, err)
			}
			output, _ := os.ReadFile(path)
			if !bytes.Equal(input, output) {
				t.Fatal("native failure changed original")
			}
		})
	}
}
