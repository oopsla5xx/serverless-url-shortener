package shortener

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type Request struct {
	OriginalURL string `json:"original_url"`
}

type Response struct {
	Code     string `json:"code"`
	ShortURL string `json:"short_url"`
}

const (
	charset    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLength = 7
	maxRetries = 5
	maxURLLen  = 2000
)

type Putter interface {
	PutObject(ctx context.Context, in *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type Service struct {
	S3      Putter
	Bucket  string
	BaseURL string
}

func code(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}

func validateURL(raw string) error {
	if raw == "" || len(raw) > maxURLLen {
		return fmt.Errorf("original_url is empty or longer than %d chars", maxURLLen)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid original_url: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("original_url must be an absolute http/https URL")
	}
	return nil
}

func isConflict(err error) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		c := ae.ErrorCode()
		return c == "PreconditionFailed" || c == "ConditionalRequestConflict"
	}
	return false
}

func (s *Service) Shorten(ctx context.Context, req Request) (Response, error) {
	if err := validateURL(req.OriginalURL); err != nil {
		return Response{}, err
	}

	for i := 0; i < maxRetries; i++ {
		key, err := code(codeLength)
		if err != nil {
			return Response{}, err
		}

		_, err = s.S3.PutObject(ctx, &s3.PutObjectInput{
			Bucket:                  aws.String(s.Bucket),
			Key:                     aws.String(key),
			WebsiteRedirectLocation: aws.String(req.OriginalURL),
			IfNoneMatch:             aws.String("*"),
		})
		if err == nil {
			log.Printf("created %s -> %s", key, req.OriginalURL)
			return Response{Code: key, ShortURL: s.BaseURL + "/" + key}, nil
		}
		if isConflict(err) {
			continue
		}
		log.Printf("PutObject failed: %v", err)
		return Response{}, err
	}

	return Response{}, fmt.Errorf("could not generate a unique code after %d attempts", maxRetries)
}
