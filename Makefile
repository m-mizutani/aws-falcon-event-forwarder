STACK_CONFIG ?= param.cfg

ifeq (,$(wildcard $(STACK_CONFIG)))
    $(error $(STACK_CONFIG) is not found)
endif

CODE_S3_BUCKET := $(shell cat $(STACK_CONFIG) | grep CodeS3Bucket | cut -d = -f 2)
CODE_S3_PREFIX := $(shell cat $(STACK_CONFIG) | grep CodeS3Prefix | cut -d = -f 2)
STACK_NAME := $(shell cat $(STACK_CONFIG) | grep StackName | cut -d = -f 2)
PARAMETERS := $(shell cat $(STACK_CONFIG) | grep -e LambdaRoleArn -e S3Bucket -e S3Prefix -e S3Region -e SecretArn | tr '\n' ' ')
TEMPLATE_FILE=template.yml
FUNCTION=build/main

all: test

build/main: *.go
	env GOARCH=amd64 GOOS=linux go build -o build/main

test: *.go
	go test -v

clean:
	rm $(FUNCTION)

sam.yml: $(TEMPLATE_FILE) $(FUNCTION)
	aws cloudformation package \
		--template-file $(TEMPLATE_FILE) \
		--s3-bucket $(CODE_S3_BUCKET) \
		--s3-prefix $(CODE_S3_PREFIX) \
		--output-template-file sam.yml

deploy: sam.yml
	aws cloudformation deploy \
		--template-file sam.yml \
		--stack-name $(STACK_NAME) \
		--capabilities CAPABILITY_IAM \
		--parameter-overrides $(PARAMETERS)
