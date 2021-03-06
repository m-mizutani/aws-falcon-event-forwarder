package main_test

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"testing"

	falcon "github.com/m-mizutani/aws-falcon-event-forwarder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadArgument() falcon.Argument {
	fd, err := os.Open("test.json")
	if err != nil {
		log.Fatal("Fail to open test parameter file: ", err)
	}

	defer fd.Close()
	data, err := ioutil.ReadAll(fd)
	if err != nil {
		log.Fatal("Fail to read parameter file: ", err)
	}

	args := falcon.Argument{}
	err = json.Unmarshal(data, &args)
	if err != nil {
		log.Fatal("Fail to unmarshal parameter file: ", err)
	}

	return args
}

func TestBasic(t *testing.T) {
	args := loadArgument()

	assert.NotEqual(t, args.S3Bucket, "")

	_, err := falcon.Handler(args)
	require.NoError(t, err)
	// assert.NotEqual(t, 0, len(resp.Uploaded))
}

func TestSignature(t *testing.T) {
	date := "Mon, 03 Dec 2018 07:40:53 GMT"
	url := "firehose.crowdstrike.com/sensors/entities/datafeed/v1"
	apiKey := "fTZk61mwcaPbmXlWQKHn"
	qs := "appId=Test"
	sig := falcon.MakeSignature("GET", date, url, qs, apiKey)
	assert.Equal(t, "9dPbDzEV7g3M8-zA93PdXwFre0pI1jLx6Lf-J3eu9ks=", sig)
}
