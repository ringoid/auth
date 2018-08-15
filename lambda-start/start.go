package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"net/http"
	"fmt"
	"io/ioutil"
	"strings"
	"os"
	"../sys_log"
	"github.com/aws/aws-lambda-go/events"
	"github.com/ringoid/auth/apimodel"
	"encoding/json"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/satori/go.uuid"
	"time"
	"strconv"
	"github.com/aws/aws-sdk-go/service/firehose"
)

var anlogger *syslog.Logger
var twilioKey string
var awsDbClient *dynamodb.DynamoDB
var tmpTokenTableName string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string

const (
	region     = "eu-west-2"
	maxRetries = 3

	twilioApiKeyName    = "twilio-api-key"
	twilioSecretKeyBase = "%s/Twilio/Api/Key"
)

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session
	var twilioSecretKeyName string

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("start.go : env can not be empty ENV")
		os.Exit(1)
	}
	fmt.Printf("start.go : start with ENV = %s", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("start.go : env can not be empty PAPERTRAIL_LOG_ADDRESS")
		os.Exit(1)
	}
	fmt.Printf("start.go : start with PAPERTRAIL_LOG_ADDRESS = %s", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "start-auth"))
	if err != nil {
		fmt.Errorf("start.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Infoln("start.go : logger was successfully initialized")

	tmpTokenTableName, ok = os.LookupEnv("TMP_TOKEN_TABLE")
	if !ok {
		fmt.Printf("start.go : env can not be empty TMP_TOKEN_TABLE")
		os.Exit(1)
	}
	anlogger.Infof("start.go : start with TMP_TOKEN_TABLE = %s", tmpTokenTableName)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(region).WithMaxRetries(maxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf("start.go : error during initialization : %v", err)
	}
	anlogger.Infoln("start.go : aws session was successfully initialized")

	twilioSecretKeyName = fmt.Sprintf(twilioSecretKeyBase, env)
	svc := secretsmanager.New(awsSession)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(twilioSecretKeyName),
	}

	result, err := svc.GetSecretValue(input)
	if err != nil {
		anlogger.Fatalf("start.go : error reading %s secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	var secretMap map[string]string
	decoder := json.NewDecoder(strings.NewReader(*result.SecretString))
	err = decoder.Decode(&secretMap)
	if err != nil {
		anlogger.Fatalf("start.go : error decode %s secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	twilioKey, ok = secretMap[twilioApiKeyName]
	if !ok {
		anlogger.Fatalln("start.go : Twilio Api Key is empty")
	}
	anlogger.Infoln("start.go : Twilio Api Key was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Infoln("start.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf("start.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Infof("start.go : start with DELIVERY_STREAM = %s", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Infoln("start.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	anlogger.Infof("start.go : handle request %v", request)

	reqParam, ok := parseParams(request.Body)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.WrongRequestParamsClientError}, nil
	}

	sourceIp := request.RequestContext.Identity.SourceIP

	event := apimodel.NewUserAcceptTermsEvent(*reqParam, sourceIp)
	sendAnalyticEvent(event)

	resp := apimodel.AuthResp{}
	ok, resp.SessionId = accessToken(reqParam.CountryCode, reqParam.Phone)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}

	ok, errorStr := startVerify(reqParam.CountryCode, reqParam.Phone)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errorStr}, nil
	}

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf("start.go : error while marshaling resp object : %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	//return OK with AccessToken
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

func parseParams(params string) (*apimodel.StartReq, bool) {
	var req apimodel.StartReq
	err := json.Unmarshal([]byte(params), &req)

	if err != nil {
		anlogger.Errorf("start.go : error parsing required params : %v", err)
		return nil, false
	}

	if req.CountryCode == 0 || req.Phone == "" || req.Device == "" || req.Os == "" || req.Screen == "" {
		anlogger.Errorf("start.go : one of the required param is nil, req %v", req)
		return nil, false
	}

	return &req, true
}

func accessToken(code int, number string) (bool, string) {
	token, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf("start.go : error while generate uuid for token : %v", err)
		return false, ""
	}

	input := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"token": {
				S: aws.String(token.String()),
			},
			"code": {
				N: aws.String(strconv.Itoa(code)),
			},
			"phone": {
				S: aws.String(number),
			},
			"ttl": {
				N: aws.String(strconv.FormatInt(time.Now().Add(5 * time.Minute).Unix(), 10)),
			},
		},
		TableName: aws.String(tmpTokenTableName),
	}

	_, err = awsDbClient.PutItem(input)
	if err != nil {
		anlogger.Errorf("start.go : error while writing tmp token : %v", err)
		return false, ""
	}
	return true, token.String()
}

func sendAnalyticEvent(event interface{}) {
	data, err := json.Marshal(event)
	if err != nil {
		anlogger.Errorf("start.go : error marshaling analytics event : %v", err)
		return
	}
	newLine := "\n"
	data = append(data, newLine...)
	_, err = awsDeliveryStreamClient.PutRecord(&firehose.PutRecordInput{
		DeliveryStreamName: aws.String(deliveryStreamName),
		Record: &firehose.Record{
			Data: data,
		},
	})

	if err != nil {
		anlogger.Errorf("start.go : error sending analytics event : %v", err)
	}
}

func startVerify(code int, number string) (bool, string) {
	params := fmt.Sprintf("via=sms&&phone_number=%s&&country_code=%d", number, code)
	req, err := http.NewRequest("POST",
		"https://api.authy.com/protected/json/phones/verification/start",
		strings.NewReader(params))

	if err != nil {
		anlogger.Errorf("start.go : error while construct the request : %v", err)
		return false, apimodel.InternalServerError
	}

	req.Header.Set("X-Authy-API-Key", twilioKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		anlogger.Errorf("start.go error while making request : %v", err)
		return false, apimodel.InternalServerError
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		anlogger.Errorf("start.go : error while sending sms, status %v, body %v",
			resp.StatusCode, string(body))
		//The only reason - wrong phone number
		if resp.StatusCode == 400 {
			return false, apimodel.PhoneNumberClientError
		}
		return false, apimodel.InternalServerError
	}

	anlogger.Infof("start.go : sms was successfully sent, status %v, body %v",
		resp.StatusCode, string(body))
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
