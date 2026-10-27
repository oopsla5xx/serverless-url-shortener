package shortener

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func urlReq(method, key, body string) events.LambdaFunctionURLRequest {
	r := events.LambdaFunctionURLRequest{Headers: map[string]string{}, Body: body}
	r.RequestContext.HTTP.Method = method
	if key != "" {
		r.Headers[APIKeyHeader] = key
	}
	return r
}

func TestHandle(t *testing.T) {
	ctx := context.Background()
	const key = "s3cret"
	okBody := `{"original_url":"https://example.com"}`

	cases := []struct {
		name    string
		apiKey  string // key configured on the Lambda
		req     events.LambdaFunctionURLRequest
		s3Err   error
		status  int
		s3Calls int
	}{
		{"missing key", key, urlReq("POST", "", okBody), nil, 401, 0},
		{"wrong key", key, urlReq("POST", "nope", okBody), nil, 401, 0},
		{"server key unset", "", urlReq("POST", "", okBody), nil, 401, 0},
		{"wrong method", key, urlReq("GET", key, okBody), nil, 405, 0},
		{"bad json", key, urlReq("POST", key, "{"), nil, 400, 0},
		{"invalid url", key, urlReq("POST", key, `{"original_url":"ftp://x"}`), nil, 400, 0},
		{"s3 error", key, urlReq("POST", key, okBody), errors.New("boom"), 500, 1},
		{"created", key, urlReq("POST", key, okBody), nil, 201, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeS3{}
			if c.s3Err != nil {
				f.errs = []error{c.s3Err}
			}
			h := &HTTPHandler{Service: &Service{S3: f, BaseURL: "https://s.example"}, APIKey: c.apiKey}
			res, err := h.Handle(ctx, c.req)
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != c.status || len(f.calls) != c.s3Calls {
				t.Fatalf("status=%d calls=%d body=%s", res.StatusCode, len(f.calls), res.Body)
			}
		})
	}

	t.Run("base64 body returns short url", func(t *testing.T) {
		req := urlReq("POST", key, base64.StdEncoding.EncodeToString([]byte(okBody)))
		req.IsBase64Encoded = true
		h := &HTTPHandler{Service: &Service{S3: &fakeS3{}, BaseURL: "https://s.example"}, APIKey: key}
		res, _ := h.Handle(ctx, req)
		var out Response
		if err := json.Unmarshal([]byte(res.Body), &out); err != nil || res.StatusCode != 201 || out.ShortURL != "https://s.example/"+out.Code {
			t.Fatalf("status=%d body=%s", res.StatusCode, res.Body)
		}
	})
}
