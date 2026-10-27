-include .env

FUNCTION   ?= url-shortener
ARCH       ?= arm64
BUILD_DIR  ?= build
URL        ?= https://example.com

BIN := $(BUILD_DIR)/bootstrap
ZIP := $(BUILD_DIR)/function.zip

.DEFAULT_GOAL := help
.PHONY: help fmt vet test check build zip deploy config invoke clean

help:
	@grep -hE '^[a-z]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-8s %s\n", $$1, $$2}'

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

check: vet test

build:
	GOOS=linux GOARCH=$(ARCH) CGO_ENABLED=0 go build -tags lambda.norpc -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/shortener

zip: build
	cd $(BUILD_DIR) && rm -f function.zip && zip -q function.zip bootstrap

deploy: check zip
	aws lambda update-function-code --function-name $(FUNCTION) --zip-file fileb://$(ZIP) --architectures $(ARCH) --query 'LastUpdateStatus' --output text
	aws lambda wait function-updated-v2 --function-name $(FUNCTION)

config:
	@test -n "$(BUCKET)" -a -n "$(BASE_URL)" || (echo "BUCKET/BASE_URL chưa set trong .env" >&2; exit 1)
	aws lambda update-function-configuration --function-name $(FUNCTION) \
		--environment '{"Variables":{"BUCKET":"$(BUCKET)","BASE_URL":"$(BASE_URL)"}}' \
		--query 'LastUpdateStatus' --output text
	aws lambda wait function-updated-v2 --function-name $(FUNCTION)

invoke:
	aws lambda invoke --function-name $(FUNCTION) --cli-binary-format raw-in-base64-out \
		--payload '{"original_url":"$(URL)"}' /dev/stdout

clean:
	rm -rf $(BUILD_DIR)
