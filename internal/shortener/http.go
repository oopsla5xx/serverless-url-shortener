package shortener

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

const APIKeyHeader = "x-api-key"

type HTTPHandler struct {
	Service *Service
	APIKey  string
}

func (h *HTTPHandler) Handle(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	if h.APIKey == "" || subtle.ConstantTimeCompare([]byte(req.Headers[APIKeyHeader]), []byte(h.APIKey)) != 1 {
		return jsonError(http.StatusUnauthorized, "unauthorized"), nil
	}
	if req.RequestContext.HTTP.Method != http.MethodPost {
		return jsonError(http.StatusMethodNotAllowed, "method not allowed"), nil
	}

	body := []byte(req.Body)
	if req.IsBase64Encoded {
		b, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return jsonError(http.StatusBadRequest, "invalid body encoding"), nil
		}
		body = b
	}

	var in Request
	if err := json.Unmarshal(body, &in); err != nil {
		return jsonError(http.StatusBadRequest, "invalid JSON body"), nil
	}

	out, err := h.Service.Shorten(ctx, in)
	switch {
	case errors.Is(err, ErrInvalidURL):
		return jsonError(http.StatusBadRequest, err.Error()), nil
	case err != nil:
		return jsonError(http.StatusInternalServerError, "internal error"), nil
	}
	return jsonResponse(http.StatusCreated, out), nil
}

func jsonResponse(status int, v any) events.LambdaFunctionURLResponse {
	b, _ := json.Marshal(v)
	return events.LambdaFunctionURLResponse{
		StatusCode: status,
		Headers:    map[string]string{"content-type": "application/json"},
		Body:       string(b),
	}
}

func jsonError(status int, msg string) events.LambdaFunctionURLResponse {
	return jsonResponse(status, map[string]string{"error": msg})
}
