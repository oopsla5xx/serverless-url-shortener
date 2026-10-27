package shortener

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// fakeS3 trả lần lượt các lỗi trong errs, hết danh sách thì thành công.
type fakeS3 struct {
	errs  []error
	calls []*s3.PutObjectInput
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.calls = append(f.calls, in)
	if len(f.calls) <= len(f.errs) {
		return nil, f.errs[len(f.calls)-1]
	}
	return &s3.PutObjectOutput{}, nil
}

var conflict = &smithy.GenericAPIError{Code: "PreconditionFailed"}

func TestShorten(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects invalid url", func(t *testing.T) {
		for _, u := range []string{"", "ftp://x.com", "/relative", "https://"} {
			f := &fakeS3{}
			if _, err := (&Service{S3: f}).Shorten(ctx, Request{OriginalURL: u}); err == nil {
				t.Errorf("%q: expected error", u)
			}
			if len(f.calls) != 0 {
				t.Errorf("%q: S3 must not be called", u)
			}
		}
	})

	t.Run("retries on conflict then succeeds", func(t *testing.T) {
		f := &fakeS3{errs: []error{conflict, conflict}}
		svc := &Service{S3: f, Bucket: "b", BaseURL: "https://s.example"}
		res, err := svc.Shorten(ctx, Request{OriginalURL: "https://example.com/a"})
		if err != nil {
			t.Fatal(err)
		}
		if len(f.calls) != 3 || len(res.Code) != codeLength || res.ShortURL != "https://s.example/"+res.Code {
			t.Fatalf("calls=%d res=%+v", len(f.calls), res)
		}
		last := f.calls[2]
		if aws.ToString(last.IfNoneMatch) != "*" || aws.ToString(last.WebsiteRedirectLocation) != "https://example.com/a" {
			t.Fatalf("bad put input: %+v", last)
		}
	})

	t.Run("gives up after max retries", func(t *testing.T) {
		f := &fakeS3{errs: []error{conflict, conflict, conflict, conflict, conflict}}
		if _, err := (&Service{S3: f}).Shorten(ctx, Request{OriginalURL: "https://example.com"}); err == nil || len(f.calls) != maxRetries {
			t.Fatalf("err=%v calls=%d", err, len(f.calls))
		}
	})

	t.Run("returns non-conflict error immediately", func(t *testing.T) {
		f := &fakeS3{errs: []error{errors.New("boom")}}
		if _, err := (&Service{S3: f}).Shorten(ctx, Request{OriginalURL: "https://example.com"}); err == nil || len(f.calls) != 1 {
			t.Fatalf("err=%v calls=%d", err, len(f.calls))
		}
	})
}
