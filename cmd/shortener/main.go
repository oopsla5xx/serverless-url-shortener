package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"serverless-url-shortener/internal/shortener"
)

func main() {
	bucket := os.Getenv("BUCKET")
	baseURL := strings.TrimRight(os.Getenv("BASE_URL"), "/")
	if bucket == "" || baseURL == "" {
		log.Fatal("missing required env: BUCKET, BASE_URL")
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	svc := &shortener.Service{S3: s3.NewFromConfig(cfg), Bucket: bucket, BaseURL: baseURL}
	lambda.Start(svc.Shorten)
}
