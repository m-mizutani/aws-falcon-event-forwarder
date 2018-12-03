package main

import (
	// "github.com/aws/aws-sdk-go/aws/client"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"

	"fmt"

	"github.com/aws/aws-sdk-go/service/secretsmanager"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Result struct {
	Uploaded []string `json:"uploaded"`
}
type lambdaEvent struct{}

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.InfoLevel)
	lambda.Start(handleRequest)
}

type Argument struct {
	S3Bucket    string `json:"s3bucket"`
	S3Prefix    string `json:"s3prefix"`
	S3Region    string `json:"s3region"`
	SecretArn   string `json:"secret_arn"`
	StateTable  string `json:"state_table"`
	TableRegion string `json:"table_region"`
	FuncName    string
}

func newArgument() Argument {
	return Argument{
		S3Bucket:    os.Getenv("S3_BUCKET"),
		S3Prefix:    os.Getenv("S3_PREFIX"),
		S3Region:    os.Getenv("S3_REGION"),
		SecretArn:   os.Getenv("SECRET_ARN"),
		FuncName:    os.Getenv("AWS_LAMBDA_FUNCTION_NAME"),
		StateTable:  os.Getenv("STATE_TABLE"),
		TableRegion: os.Getenv("AWS_REGION"),
	}
}

type secretValues struct {
	FalconUUID string `json:"falcon_uuid"`
	FalconKey  string `json:"falcon_key"`
}

func Handler(args Argument) (result Result, err error) {
	log.WithField("args", args).Info("Start")
	var secrets secretValues
	err = getSecretValues(args.SecretArn, &secrets)
	if err != nil {
		return
	}

	client := NewFalconClient(args.TableRegion, args.StateTable,
		secrets.FalconUUID, secrets.FalconKey, args.FuncName)

	err = client.query()
	if err != nil {
		return
	}

	return
}

func hasS3File(s3Region, s3Bucket, s3Key string) (bool, error) {
	ssn := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(s3Region),
	}))

	input := &s3.HeadObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(s3Key),
	}

	s3service := s3.New(ssn)
	log.WithField("input", input).Info("try to check if the file exists")
	_, err := s3service.HeadObject(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			return false, nil
		}

		log.WithField("error", err).Error("Fail Head Object")
		return false, errors.Wrap(err, "Fail to check if S3 obejct exists")
	}

	return true, nil
}

func uploadS3File(s3Region, s3Bucket, s3Key string, data []byte) error {
	// Upload
	body := bytes.NewReader(data)

	log.WithField("s3key", s3Key).Info("try to upload")
	ssn := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(s3Region),
	}))
	uploader := s3manager.NewUploader(ssn)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(s3Key),
		Body:   body,
	})
	if err != nil {
		return errors.Wrap(err, "Fail to upload data to your bucket")
	}

	return nil
}

func handleRequest(ctx context.Context, event lambdaEvent) (Result, error) {
	opts := newArgument()
	return Handler(opts)
}

func getSecretValues(secretArn string, values interface{}) error {
	// sample: arn:aws:secretsmanager:ap-northeast-1:1234567890:secret:mytest
	arn := strings.Split(secretArn, ":")
	if len(arn) != 7 {
		return errors.New(fmt.Sprintf("Invalid SecretsManager ARN format: %s", secretArn))
	}
	region := arn[3]

	ssn := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))
	mgr := secretsmanager.New(ssn)

	result, err := mgr.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretArn),
	})

	if err != nil {
		return errors.Wrap(err, "Fail to retrieve secret values")
	}

	err = json.Unmarshal([]byte(*result.SecretString), values)
	if err != nil {
		return errors.Wrap(err, "Fail to parse secret values as JSON")
	}
	log.Info("Feteched secrets data")

	return nil
}
