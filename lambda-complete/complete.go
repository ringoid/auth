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
	"github.com/aws/aws-lambda-go/events"
	"net/http"
	"io/ioutil"
	"github.com/satori/go.uuid"
	"time"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

var anlogger *syslog.Logger
var twilioKey string
var awsDbClient *dynamodb.DynamoDB
var userTableName string
var userProfileTable string
var neo4jurl string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string

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
	anlogger.Debugf(nil, "complete.go : logger was successfully initialized")

	userTableName, ok = os.LookupEnv("USER_TABLE")
	if !ok {
		fmt.Printf("complete.go : env can not be empty USER_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "complete.go : start with USER_TABLE = [%s]", userTableName)

	neo4jurl, ok = os.LookupEnv("NEO4J_URL")
	if !ok {
		fmt.Printf("complete.go : env can not be empty NEO4J_URL")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "complete.go : start with NEO4J_URL = [%s]", neo4jurl)

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("complete.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "complete.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "complete.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "complete.go : aws session was successfully initialized")

	twilioSecretKeyName = fmt.Sprintf(apimodel.TwilioSecretKeyBase, env)
	svc := secretsmanager.New(awsSession)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(twilioSecretKeyName),
	}

	result, err := svc.GetSecretValue(input)
	if err != nil {
		anlogger.Fatalf(nil, "complete.go : error reading %s secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	var secretMap map[string]string
	decoder := json.NewDecoder(strings.NewReader(*result.SecretString))
	err = decoder.Decode(&secretMap)
	if err != nil {
		anlogger.Fatalf(nil, "complete.go : error decode %s secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	twilioKey, ok = secretMap[apimodel.TwilioApiKeyName]
	if !ok {
		anlogger.Fatalln(nil, "complete.go : Twilio Api Key is empty")
	}
	anlogger.Debugf(nil, "complete.go : Twilio Api Key was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "complete.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "complete.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "complete.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "complete.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "complete.go : handle request %v", request)

	reqParam, ok := parseParams(request.Body, lc)
	if !ok {
		anlogger.Errorf(lc, "complete.go : return %s to client", apimodel.WrongRequestParamsClientError)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.WrongRequestParamsClientError}, nil
	}

	userInfo, ok, errStr := fetchBySessionId(reqParam.SessionId, lc)
	if !ok {
		anlogger.Errorf(lc, "complete.go : userId [%s], return %s to client", userInfo.UserId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	ok, errStr = completeVerify(userInfo, reqParam.VerificationCode, lc)
	if !ok {
		anlogger.Errorf(lc, "complete.go : userId [%s], return %s to client", userInfo.UserId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	event := apimodel.NewUserVerificationCompleteEvent(userInfo.UserId)
	apimodel.SendAnalyticEvent(event, userInfo.UserId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	accessToken, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "complete.go : error while generate accessToken for userId [%s] : %v", userInfo.UserId, err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}

	userExist, ok, errStr := updateAccessToke(userInfo.UserId, accessToken.String(), lc)
	if !ok {
		anlogger.Errorf(lc, "complete.go : userId [%s], return %s to client", userInfo.UserId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	resp := apimodel.VerifyResp{AccessToken: accessToken.String(), AccountAlreadyExist: userExist}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "complete.go : error while marshaling resp object %v for userId [%s] : %v", resp, userInfo.UserId, err)
		anlogger.Errorf(lc, "complete.go : userId [%s], return %s to client", userInfo.UserId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	//return OK with AccessToken
	anlogger.Debugf(lc, "complete.go : return body=%s to client, userId [%s]", string(body), userInfo.UserId)
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

//return do we already have such user, ok, errorString if not ok
func updateAccessToke(userId, accessToken string, lc *lambdacontext.LambdaContext) (bool, bool, string) {
	anlogger.Debugf(lc, "complete.go : update accessToken [%s] for userId [%s]", accessToken, userId)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#token":     aws.String(apimodel.AccessTokenColumnName),
			"#updatedAt": aws.String(apimodel.TokenUpdatedTimeColumnName),
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
			apimodel.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		ReturnValues:     aws.String("ALL_OLD"),
		TableName:        aws.String(userProfileTable),
		UpdateExpression: aws.String("SET #token = :tV, #updatedAt = :uV"),
	}

	result, err := awsDbClient.UpdateItem(input)

	if err != nil {
		anlogger.Errorf(lc, "complete.go : error update accessToken [%s] for userId [%s] : %v", accessToken, userId, err)
		return false, false, apimodel.InternalServerError
	}

	_, ok := result.Attributes[apimodel.SexColumnName]

	anlogger.Debugf(lc, "complete.go : successfully update accessToken [%s] for userId [%s]", accessToken, userId)
	return ok, true, ""
}

//return ok and error string if not
func completeVerify(userInfo *apimodel.UserInfo, verificationCode string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "complete.go : verify phone for userId [%s], userInfo=%v", userInfo.UserId, userInfo)

	url := fmt.Sprintf("https://api.authy.com/protected/json/phones/verification/check?phone_number=%s&country_code=%d&verification_code=%s",
		userInfo.PhoneNumber, userInfo.CountryCode, verificationCode)

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		anlogger.Errorf(lc, "complete.go : error while construct the request, userId [%s] : %v", userInfo.UserId, err)
		return false, apimodel.InternalServerError
	}

	req.Header.Set("X-Authy-API-Key", twilioKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}

	anlogger.Debugf(lc, "complete.go : make GET request by url %s, userId [%s]", url, userInfo.UserId)

	resp, err := client.Do(req)
	if err != nil {
		anlogger.Fatalf(lc, "complete.go : error while making GET request, userId [%s] : %v", userInfo.UserId, err)
		return false, apimodel.InternalServerError
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		anlogger.Errorf(lc, "complete.go : error reading response body from Twilio, userId [%s] : %v", userInfo.UserId, err)
		return false, apimodel.InternalServerError
	}
	anlogger.Debugf(lc, "complete.go : receive response from Twilio, body=%s, userId [%s]", string(body), userInfo.UserId)
	if resp.StatusCode != 200 {
		anlogger.Errorf(lc, "complete.go : error while sending sms, status %v, body %v, userId [%s]",
			resp.StatusCode, string(body), userInfo.UserId)

		var errorResp map[string]interface{}
		err := json.Unmarshal(body, &errorResp)
		if err != nil {
			anlogger.Errorf(lc, "complete.go : error parsing Twilio response, body=%s, userId [%s] : %v", body, userInfo.UserId, err)
			return false, apimodel.InternalServerError
		}

		if errorCodeObject, ok := errorResp["error_code"]; ok {
			if errorCodeStr, ok := errorCodeObject.(string); ok {
				anlogger.Errorf(lc, "complete.go : error verify phone, error_code=%s, userId [%s]", errorCodeStr, userInfo.UserId)
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

	anlogger.Infof(lc, "complete.go : successfully complete verify for userId [%s], userInfo=%v",
		userInfo.UserId, userInfo)
	return true, ""
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.VerifyReq, bool) {
	var req apimodel.VerifyReq
	err := json.Unmarshal([]byte(params), &req)

	if err != nil {
		anlogger.Errorf(lc, "complete.go : error unmarshal required params from the string %s : %v", params, err)
		return nil, false
	}

	if req.SessionId == "" || req.VerificationCode == "" {
		anlogger.Errorf(lc, "complete.go : one of the required param is nil or empty, req %v", req)
		return nil, false
	}

	return &req, true
}

//return userInfo, is everything ok and error string if not
func fetchBySessionId(sessionId string, lc *lambdacontext.LambdaContext) (*apimodel.UserInfo, bool, string) {
	anlogger.Debugf(lc, "complete.go : fetch userInfo by sessionId [%s]", sessionId)

	input := &dynamodb.QueryInput{
		ExpressionAttributeNames: map[string]*string{
			"#sessionId": aws.String(apimodel.SessionIdColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":sV": {
				S: aws.String(sessionId),
			},
		},
		KeyConditionExpression: aws.String("#sessionId = :sV"),
		IndexName:              aws.String(apimodel.SessionGSIName),
		TableName:              aws.String(userTableName),
	}

	res, err := awsDbClient.Query(input)

	if err != nil {
		anlogger.Errorf(lc, "complete.go : error while fetch userInfo by sessionId [%s] : %v", sessionId, err)
		return &apimodel.UserInfo{}, false, apimodel.InternalServerError
	}

	if len(res.Items) == 0 {
		anlogger.Warnf(lc, "complete.go : wrong sessionId [%s], there is no userInfo with such sessionId", sessionId)
		return &apimodel.UserInfo{}, false, apimodel.WrongSessionIdClientError
	}

	if len(res.Items) != 1 {
		anlogger.Errorf(lc, "complete.go : error several userInfo by one sessionId [%s], result=%v", sessionId, res.Items)
		return &apimodel.UserInfo{}, false, apimodel.InternalServerError
	}
	userId := *res.Items[0][apimodel.UserIdColumnName].S
	sessId := *res.Items[0][apimodel.SessionIdColumnName].S
	phone := *res.Items[0][apimodel.PhoneColumnName].S
	phonenumber := *res.Items[0][apimodel.PhoneNumberColumnName].S

	countryCode, err := strconv.Atoi(*res.Items[0][apimodel.CountryCodeColumnName].S)
	if err != nil {
		anlogger.Errorf(lc, "complete.go : error while parsing country code, sessionId [%s] : %v", sessionId, err)
		return &apimodel.UserInfo{}, false, apimodel.InternalServerError
	}
	customerId := *res.Items[0][apimodel.CustomerIdColumnName].S

	userInfo := &apimodel.UserInfo{
		UserId:      userId,
		SessionId:   sessId,
		Phone:       phone,
		CountryCode: countryCode,
		PhoneNumber: phonenumber,
		CustomerId:  customerId,
	}

	anlogger.Debugf(lc, "complete.go : successfully fetch userInfo %v by sessionId [%s]", userInfo, sessionId)

	return userInfo, true, ""
}

func main() {
	basicLambda.Start(handler)
}
