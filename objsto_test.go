package objsto_test

//go:generate moq -out mock_test.go -pkg objsto_test . HttpDoer Logger

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/objsto"
)

func TestObjSto(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ObjSto Suite")
}

var _ = Describe("Client", func() {
	var (
		ctx    = context.Background()
		cfg    *objsto.Config
		mock   *HttpDoerMock
		client *objsto.Client
		lgr    *LoggerMock
	)

	BeforeEach(func() {
		cfg = &objsto.Config{
			Region:    "test-region",
			Scheme:    "https",
			Host:      "test-host",
			Bucket:    "test-bucket",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
		}

		mock = &HttpDoerMock{}
		lgr = &LoggerMock{
			InfoFunc:  func(ctx context.Context, msg string, kv ...any) {},
			DebugFunc: func(ctx context.Context, msg string, kv ...any) {},
			TraceFunc: func(ctx context.Context, msg string, kv ...any) {},
			ErrorFunc: func(ctx context.Context, msg string, err error, kv ...any) {},
		}

		client = cfg.New(mock, lgr)
	})

	Describe("Get", func() {
		var (
			object string
			reader io.ReadCloser
			err    error
		)

		JustBeforeEach(func() {
			reader, err = client.Get(ctx, object)
		})

		When("object is blank", func() {
			BeforeEach(func() {
				object = ""
			})

			It("returns error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot be blank"))
			})
		})

		When("request succeeds", func() {
			BeforeEach(func() {
				object = "test-object.txt"
				mock.DoFunc = func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewReader([]byte("test content"))),
					}, nil
				}
			})

			It("returns reader with body", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(reader).ToNot(BeNil())

				content, _ := io.ReadAll(reader)
				Expect(string(content)).To(Equal("test content"))
			})

			It("sends signed request to correct path", func() {
				calls := mock.DoCalls()
				Expect(calls).To(HaveLen(1))
				Expect(calls[0].Request.URL.Path).To(Equal("/test-bucket/test-object.txt"))
				Expect(calls[0].Request.Header.Get("Authorization")).To(ContainSubstring("AWS4-HMAC-SHA256"))
			})
		})
	})

	Describe("Put", func() {
		var (
			object string
			body   io.ReadSeeker
			err    error
		)

		JustBeforeEach(func() {
			err = client.Put(ctx, object, body)
		})

		When("object is blank", func() {
			BeforeEach(func() {
				object = ""
				body = bytes.NewReader([]byte("data"))
			})

			It("returns error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot be blank"))
			})
		})

		When("request succeeds", func() {
			BeforeEach(func() {
				object = "test-object.txt"
				body = bytes.NewReader([]byte("upload content"))
				mock.DoFunc = func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewReader(nil)),
					}, nil
				}
			})

			It("succeeds without error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("sends PUT request with body hash", func() {
				calls := mock.DoCalls()
				Expect(calls).To(HaveLen(1))
				Expect(calls[0].Request.Method).To(Equal("PUT"))
				Expect(calls[0].Request.Header.Get("x-amz-content-sha256")).ToNot(BeEmpty())
			})
		})
	})

	Describe("List", func() {
		var (
			prefix string
			keys   []string
			err    error
		)

		JustBeforeEach(func() {
			keys, err = client.List(ctx, prefix)
		})

		When("request succeeds with objects", func() {
			BeforeEach(func() {
				prefix = "export_1_"
				mock.DoFunc = func(req *http.Request) (*http.Response, error) {
					xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Contents><Key>export_1_20260227.json.gz</Key></Contents>
  <Contents><Key>export_1_20260228.json.gz</Key></Contents>
</ListBucketResult>`
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewReader([]byte(xmlBody))),
					}, nil
				}
			})

			It("returns the keys", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(keys).To(Equal([]string{
					"export_1_20260227.json.gz",
					"export_1_20260228.json.gz",
				}))
			})

			It("sends request with correct query params", func() {
				calls := mock.DoCalls()
				Expect(calls).To(HaveLen(1))
				Expect(calls[0].Request.URL.RawQuery).To(ContainSubstring("list-type=2"))
				Expect(calls[0].Request.URL.RawQuery).To(ContainSubstring("prefix=export_1_"))
			})
		})

		When("request succeeds with no objects", func() {
			BeforeEach(func() {
				prefix = "nonexistent_"
				mock.DoFunc = func(req *http.Request) (*http.Response, error) {
					xmlBody := `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
</ListBucketResult>`
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewReader([]byte(xmlBody))),
					}, nil
				}
			})

			It("returns empty slice", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(keys).To(BeEmpty())
			})
		})
	})
})
