package main

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
	"github.com/k0kubun/pp"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type falconState struct {
	FuncName       string    `dynamo:"func_name"`
	Expire         time.Time `dynamo:"expire"`
	EncryptedToken []byte    `dynamo:"encrypted_token"`
	Offset         int       `dynamo:"offset"`
}

func (x *falconState) setToken(token, key string) error {
	enc, err := aes.NewCipher([]byte(key))
	if err != nil {
		return errors.Wrap(err, "Fail to create new cipher")
	}

	rawData := []byte(token)
	encData := make([]byte, len(rawData))
	enc.Encrypt(encData, rawData)
	x.EncryptedToken = encData

	return nil
}

func (x *falconState) token(key string) string {
	enc, err := aes.NewCipher([]byte(key))
	if err != nil {
		log.WithField("error", err).Fatal("Fail to create chipher ")
	}

	rawData := make([]byte, len(x.EncryptedToken))
	enc.Decrypt(rawData, x.EncryptedToken)
	return string(rawData)
}

func (x *falconState) hasToken() bool {
	return len(x.EncryptedToken) > 0
}

func (x *falconState) setExpiration(expiration string) error {
	seg := strings.Split(expiration, ".")
	if len(seg) != 2 {
		return errors.New("Invalid time format: " + expiration)
	}

	// format 2018-12-03T09:22:00.925417304Z
	t, err := time.Parse("2006-01-02T15:04:05", seg[0])
	if err != nil {
		return errors.Wrap(err, "")
	}

	x.Expire = t
	return nil
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
	if !x.state.hasToken() {
		err := x.table.Get("func_name", x.funcName).One(&x.state)

		if err != nil {
			state, err := x.discover()
			if err != nil {
				return errors.Wrap(err, "Invalid status")
			}

			x.state = state
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

func (x *FalconClient) discover() (falconState, error) {
	method := "GET"
	var resp *http.Response
	var state falconState

	for {
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
			return state, errors.Wrap(err, "Fail to create a new HTTP request")
		}

		// headers = {"Date": Date, "Authorization": authorization}
		req.Header.Add("Authorization", auth)
		req.Header.Add("Date", date)

		resp, err = client.Do(req)
		if err != nil {
			return state, errors.Wrap(err, "HTTPS request error to "+x.BaseURL)
		}

		if resp.StatusCode == 200 {
			break
		}

		time.Sleep(time.Second * 5)
	}

	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return state, errors.Wrap(err, "Fail to read data from "+x.BaseURL)
	}

	fmt.Println(string(respData))
	var discover discoverResult
	err = json.Unmarshal(respData, &discover)
	if err != nil {
		return state, errors.Wrap(err, "Fail to unmarshal response")
	}

	pp.Println(discover)
	if len(discover.Resources) == 0 {
		log.WithField("response", string(respData)).Warn("Invalid response")
		return state, errors.New("No available resource")
	}
	state.setToken(discover.Resources[0].SessionToken.Token, x.apiKey)
	state.setExpiration(discover.Resources[0].SessionToken.Expiration)

	return state, nil
}

func (x *FalconClient) query() error {
	err := x.init()
	if err != nil {
		return err
	}

	log.Info("initialzed, ready to query")
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
