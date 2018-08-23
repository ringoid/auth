package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"../sys_log"
	"../apimodel"
	"strconv"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	"os"
	"fmt"
	"strings"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"encoding/json"
	"log"
	"github.com/aws/aws-lambda-go/events"
	"net/http"
	"io/ioutil"
	"github.com/satori/go.uuid"
	"time"
)

var anlogger *syslog.Logger
var twilioKey string
var awsDbClient *dynamodb.DynamoDB
var userTableName string
var userProfileTable string
var neo4jurl string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string

const (
	region     = "eu-west-1"
	maxRetries = 3

	twilioApiKeyName    = "twilio-api-key"
	twilioSecretKeyBase = "%s/Twilio/Api/Key"

	sessionGSIName = "sessionGSI"

	phoneColumnName     = "phone"
	userIdColumnName    = "user_id"
	sessionIdColumnName = "session_id"

	countryCodeColumnName      = "country_code"
	phoneNumberColumnName      = "phone_number"
	tokenUpdatedTimeColumnName = "token_updated_at"

	accessTokenColumnName = "access_token"
	sexColumnName         = "sex"
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
		fmt.Printf("complete.go : env can not be empty ENV")
		os.Exit(1)
	}
	fmt.Printf("complete.go : start with ENV = [%s]", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("complete.go : env can not be empty PAPERTRAIL_LOG_ADDRESS")
		os.Exit(1)
	}
	fmt.Printf("complete.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "complete-auth"))
	if err != nil {
		fmt.Errorf("complete.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf("complete.go : logger was successfully initialized")

	userTableName, ok = os.LookupEnv("USER_TABLE")
	if !ok {
		fmt.Printf("complete.go : env can not be empty USER_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf("complete.go : start with USER_TABLE = [%s]", userTableName)

	neo4jurl, ok = os.LookupEnv("NEO4J_URL")
	if !ok {
		fmt.Printf("complete.go : env can not be empty NEO4J_URL")
		os.Exit(1)
	}
	anlogger.Debugf("complete.go : start with NEO4J_URL = [%s]", neo4jurl)

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("complete.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf("complete.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(region).WithMaxRetries(maxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf("complete.go : error during initialization : %v", err)
	}
	anlogger.Debugf("complete.go : aws session was successfully initialized")

	twilioSecretKeyName = fmt.Sprintf(twilioSecretKeyBase, env)
	svc := secretsmanager.New(awsSession)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(twilioSecretKeyName),
	}

	result, err := svc.GetSecretValue(input)
	if err != nil {
		anlogger.Fatalf("complete.go : error reading %s secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	var secretMap map[string]string
	decoder := json.NewDecoder(strings.NewReader(*result.SecretString))
	err = decoder.Decode(&secretMap)
	if err != nil {
		anlogger.Fatalf("complete.go : error decode %s secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	twilioKey, ok = secretMap[twilioApiKeyName]
	if !ok {
		anlogger.Fatalln("complete.go : Twilio Api Key is empty")
	}
	anlogger.Debugf("complete.go : Twilio Api Key was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf("complete.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf("complete.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf("complete.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf("complete.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	anlogger.Debugf("complete.go : handle request %v", request)

	reqParam, ok := parseParams(request.Body)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.WrongRequestParamsClientError}, nil
	}

	userInfo, ok, errStr := fetchBySessionId(reqParam.SessionId)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	ok, errStr = completeVerify(userInfo, reqParam.VerificationCode)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	event := apimodel.NewUserVerificationCompleteEvent(userInfo.UserId)
	sendAnalyticEvent(event)

	accessToken, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf("complete.go : error while generate accessToken : %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}

	userExist, ok, errStr := updateAccessToke(userInfo.UserId, accessToken.String())
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	resp := apimodel.VerifyResp{AccessToken: accessToken.String(), AccountAlreadyExist: userExist}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf("complete.go : error while marshaling resp object : %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	//return OK with AccessToken
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

//return do we already have such user, ok, errorString if not ok
func updateAccessToke(userId, accessToken string) (bool, bool, string) {
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#token":     aws.String(accessTokenColumnName),
			"#updatedAt": aws.String(tokenUpdatedTimeColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":tV": {
				S: aws.String(accessToken),
			},
			":uV": {
				S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			userIdColumnName: {
				S: aws.String(userId),
			},
		},
		ReturnValues:     aws.String("ALL_OLD"),
		TableName:        aws.String(userProfileTable),
		UpdateExpression: aws.String("SET #token = :tV, #updatedAt = :uV"),
	}

	result, err := awsDbClient.UpdateItem(input)

	if err != nil {
		anlogger.Errorf("complete.go : error while save access token : %v", err)
		return false, false, apimodel.InternalServerError
	}

	_, ok := result.Attributes[sexColumnName]

	anlogger.Infof("complete.go : result from access to gender is %v", ok)
	return ok, true, ""
}

func sendAnalyticEvent(event interface{}) {
	data, err := json.Marshal(event)
	if err != nil {
		anlogger.Errorf("complete.go : error marshaling analytics event : %v", err)
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
		anlogger.Errorf("complete.go : error sending analytics event : %v", err)
	}
}

//return ok and error string if not
func completeVerify(userInfo *apimodel.UserInfo, verificationCode int) (bool, string) {
	url := fmt.Sprintf("https://api.authy.com/protected/json/phones/verification/check?phone_number=%s&country_code=%d&verification_code=%d",
		userInfo.PhoneNumber, userInfo.CountryCode, verificationCode)

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		anlogger.Errorf("complete.go : error while construct the request : %v", err)
		return false, apimodel.InternalServerError
	}

	req.Header.Set("X-Authy-API-Key", twilioKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}

	anlogger.Debugf("complete.go : make GET request by url %s", url)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("complete.go : error while making request : %v", err)
		return false, apimodel.InternalServerError
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		anlogger.Errorf("complete.go : error reading response body from Twilio : %v", err)
		return false, apimodel.InternalServerError
	}

	if resp.StatusCode != 200 {
		anlogger.Errorf("complete.go : error while sending sms, status %v, body %v",
			resp.StatusCode, string(body))

		var errorResp map[string]interface{}
		err := json.Unmarshal(body, &errorResp)
		if err != nil {
			anlogger.Errorf("complete.go : error parsing Twilio response : %v", err)
			return false, apimodel.InternalServerError
		}

		if errorCodeObject, ok := errorResp["error_code"]; ok {
			if errorCodeStr, ok := errorCodeObject.(string); ok {
				switch errorCodeStr {
				case "60023":
					return false, apimodel.NoPendingVerificationClientError
				case "60022":
					return false, apimodel.WrongVerificationCodeClientError
				}
			}
		}

		return false, apimodel.InternalServerError
	}

	anlogger.Infof("complete.go : successfully complete verification for user %v, response body %v",
		userInfo, string(body))
	return true, ""
}

func parseParams(params string) (*apimodel.VerifyReq, bool) {
	var req apimodel.VerifyReq
	err := json.Unmarshal([]byte(params), &req)

	if err != nil {
		anlogger.Errorf("complete.go : error parsing required params from the string %s : %v", params, err)
		return nil, false
	}

	if req.SessionId == "" || req.VerificationCode == 0 {
		anlogger.Errorf("complete.go : one of the required param is nil, req %v", req)
		return nil, false
	}

	return &req, true
}

//return userInfo, is everything ok and error string if not
func fetchBySessionId(sessionId string) (*apimodel.UserInfo, bool, string) {
	input := &dynamodb.QueryInput{
		ExpressionAttributeNames: map[string]*string{
			"#sessionId": aws.String(sessionIdColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":sV": {
				S: aws.String(sessionId),
			},
		},
		KeyConditionExpression: aws.String("#sessionId = :sV"),
		IndexName:              aws.String(sessionGSIName),
		TableName:              aws.String(userTableName),
	}

	res, err := awsDbClient.Query(input)

	if err != nil {
		anlogger.Errorf("complete.go : error while fetch userInfo by sessionId : %v", err)
		return &apimodel.UserInfo{}, false, apimodel.InternalServerError
	}

	if len(res.Items) == 0 {
		anlogger.Warnf("complete.go : wrong sessionId %s", sessionId)
		return &apimodel.UserInfo{}, false, apimodel.WrongSessionIdClientError
	}

	if len(res.Items) != 1 {
		anlogger.Errorf("complete.go : error several userInfo by one sessionId")
		return &apimodel.UserInfo{}, false, apimodel.InternalServerError
	}
	userId := *res.Items[0][userIdColumnName].S
	sessId := *res.Items[0][sessionIdColumnName].S
	phone := *res.Items[0][phoneColumnName].S
	phonenumber := *res.Items[0][phoneNumberColumnName].S

	countryCode, err := strconv.Atoi(*res.Items[0][countryCodeColumnName].S)
	if err != nil {
		anlogger.Errorf("complete.go : error while parsing country code : %v", err)
		return &apimodel.UserInfo{}, false, apimodel.InternalServerError
	}

	return &apimodel.UserInfo{
		UserId:      userId,
		SessionId:   sessId,
		Phone:       phone,
		CountryCode: countryCode,
		PhoneNumber: phonenumber,
	}, true, ""
}

func main() {
	basicLambda.Start(handler)
}
