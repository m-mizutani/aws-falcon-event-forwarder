package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
	"github.com/pkg/errors"
)

type falconState struct {
	FuncName       string    `dynamo:"func_name"`
	Expire         time.Time `dynamo:"expire"`
	EncryptedToken string    `dynamo:"encrypted_token"`
	Offset         int       `dynamo:"offset"`
}

func (x *falconState) token(key string) string {
	return ""
}

func (x *falconState) hasToken() bool {
	return (x.EncryptedToken != "")
}

type FalconClient struct {
	BaseURL       string
	CannonicalURI string
	AppID         string

	funcName string
	apiUUID  string
	apiKey   string
	state    falconState
	table    dynamo.Table
}

type discoverResource struct {
	DataFeedURL  string `json:"dataFeedURL"`
	SessionToken struct {
		Token      string `json:"token"`
		Expiration string `json:"expiration"`
	} `json:"sessionToken"`
}
type discoverResult struct {
	Resources []discoverResource `json:"resources"`
}

func NewFalconClient(tableRegion, tableName, apiUUID, apiKey, funcName string) FalconClient {
	client := FalconClient{
		BaseURL:       "https://firehose.crowdstrike.com/sensors/entities/datafeed/v1",
		CannonicalURI: "firehose.crowdstrike.com/sensors/entities/datafeed/v1",
		AppID:         "AwsFalconClient",
		funcName:      funcName,
		apiUUID:       apiUUID,
		apiKey:        apiKey,
	}

	db := dynamo.New(session.New(), &aws.Config{Region: aws.String(tableRegion)})
	client.table = db.Table(tableName)

	return client
}

func (x *FalconClient) init() error {
	if x.state.EncryptedToken == "" {
		err := x.table.Get("func_name", x.funcName).One(&x.state)
		if err != nil {
			return errors.Wrap(err, "Fail to get QueryState")
		}
	}

	return nil
}

func MakeSignature(method, date, uri, queryString, apiKey string) string {
	payload := method + "\n\n" + date + "\n" + uri + "\n" + queryString
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(payload))
	signature := mac.Sum(nil)

	return base64.URLEncoding.EncodeToString(signature)
}

func (x *FalconClient) discover() error {
	method := "GET"
	ts := time.Now().UTC()
	// date := ts.Format("Mon, 02 Jan 2006 15:04:05 -0700")
	date := ts.Format("Mon, 02 Jan 2006 15:04:05 GMT")
	fmt.Println(date)

	qs := fmt.Sprintf("appId=%s", x.AppID)
	signature := MakeSignature(method, date, x.CannonicalURI, qs, x.apiKey)
	auth := fmt.Sprintf("cs-hmac %s:%s:%s", x.apiUUID, signature, "customers")

	fmt.Printf("auth = %s\n", auth)

	client := &http.Client{}
	req, err := http.NewRequest(method, x.BaseURL+"?"+qs, nil)
	if err != nil {
		return errors.Wrap(err, "Fail to create a new HTTP request")
	}

	// headers = {"Date": Date, "Authorization": authorization}
	req.Header.Add("Authorization", auth)
	req.Header.Add("Date", date)

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "HTTPS request error to "+x.BaseURL)
	}

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "Fail to read data from "+x.BaseURL)
	}

	fmt.Println(string(respData))

	return nil
}

func (x *FalconClient) query() error {
	if !x.state.hasToken() {
		err := x.discover()
		if err != nil {
			return errors.Wrap(err, "Invalid status")
		}
	}

	/*
		method := "GET"
		token := x.state.token(x.apiKey)

		client := &http.Client{}
		req, err := http.NewRequest(method, x.BaseURL, nil)

		req.Header.Add("Authorization", fmt.Sprintf("Token %s", token))
		resp, err := client.Do(req)
		if err != nil {
			return errors.Wrap(err, "HTTPS request error to "+x.BaseURL)
		}

		respData, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "Fail to read data from "+x.BaseURL)
		}

		fmt.Println(string(respData))
	*/
	return nil
}
