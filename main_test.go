package main_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	falcon "github.com/m-mizutani/aws-falcon-event-forwarder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	fd, err := os.Open("test.json")
	require.NoError(t, err)
	defer fd.Close()
	data, err := ioutil.ReadAll(fd)

	args := falcon.NewArgument()
	err = json.Unmarshal(data, &args)
	require.NoError(t, err)

	// testCode := uuid.NewV4().String()
	// opts.S3Prefix = fmt.Sprintf("%s%s/", opts.S3Prefix, testCode)

	assert.NotEqual(t, args.S3Bucket, "")

	_, err = falcon.Handler(args)
	require.NoError(t, err)
	// assert.NotEqual(t, 0, len(resp.Uploaded))
}
