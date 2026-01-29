package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/clarktrimble/objsto"
)

// have a look at clarktrimble/launch for a little butter on top of envconfig
type config struct {
	S3        *objsto.Config `json:"s3"`
	S3Timeout time.Duration  `json:"s3_timeout" desc:"objectstore client timeout" default:"30s"`
}

func main() {
	cfg := &config{
		S3Timeout: 33 * time.Second,
		S3: &objsto.Config{
			Region:    "testoregion",
			Scheme:    "http",
			Host:      "container4:3900",
			Bucket:    "testbucket",
			AccessKey: "GKdf62cf3b0b0edb99e0eb138c",
			SecretKey: "dont wanna check this in, yeah",
		},
	}

	ctx := context.Background()

	httpClient := &http.Client{Timeout: cfg.S3Timeout}
	client := cfg.S3.New(httpClient, &subMinLog{})

	name := "demo.txt"
	data := bytes.NewReader([]byte("imapc"))

	err := client.Put(ctx, name, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("uploaded to %s\n", name)
}

// have a look at clarktrimble/sabot for contextual, structured, flat logging
type subMinLog struct{}

func (ml *subMinLog) Info(ctx context.Context, msg string, kv ...any) {
	log("info ", msg, kv)
}
func (ml *subMinLog) Debug(ctx context.Context, msg string, kv ...any) {
	log("debug", msg, kv)
}
func (ml *subMinLog) Trace(ctx context.Context, msg string, kv ...any) {
	log("trace", msg, kv)
}
func (ml *subMinLog) Error(ctx context.Context, msg string, err error, kv ...any) {
	kv = append([]any{"error", err.Error()}, kv...)
	log("error", msg, kv)
}
func log(lvl, msg string, kv []any) {
	strs := []string{}
	for _, korv := range kv {
		str, ok := korv.(string)
		if !ok || str == "" {
			str = "*"
		}
		strs = append(strs, str)
	}
	fmt.Printf("%s msg > %s  %s\n", lvl, msg, strings.Join(strs, "|"))
}
