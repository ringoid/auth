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
	"github.com/aws/aws-sdk-go/aws/session"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/satori/go.uuid"
	"time"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/dgrijalva/jwt-go"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

var anlogger *syslog.Logger
var twilioKey string
var secretWord string
var awsDbClient *dynamodb.DynamoDB
var userTableName string
var userProfileTable string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string
var nexmoApiKey string
var nexmoApiSecret string

var awsCWClient *cloudwatch.CloudWatch
var baseCloudWatchNamespace string
var nexmoCompleteMetricName string
var twilioCompleteMetricName string
var newUserWasCreatedMetricName string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : complete.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : complete.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : complete.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : complete.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "complete-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : complete.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : complete.go : logger was successfully initialized")

	userTableName, ok = os.LookupEnv("USER_TABLE")
	if !ok {
		fmt.Printf("lambda-initialization : complete.go : env can not be empty USER_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : complete.go : start with USER_TABLE = [%s]", userTableName)

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("lambda-initialization : complete.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : complete.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : complete.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : complete.go : aws session was successfully initialized")

	twilioKey = apimodel.GetSecret(fmt.Sprintf(apimodel.TwilioSecretKeyBase, env), apimodel.TwilioApiKeyName, awsSession, anlogger, nil)
	secretWord = apimodel.GetSecret(fmt.Sprintf(apimodel.SecretWordKeyBase, env), apimodel.SecretWordKeyName, awsSession, anlogger, nil)
	nexmoApiKey = apimodel.GetSecret(fmt.Sprintf(apimodel.NexmoSecretKeyBase, env), apimodel.NexmoApiKeyName, awsSession, anlogger, nil)
	nexmoApiSecret = apimodel.GetSecret(fmt.Sprintf(apimodel.NexmoApiSecretKeyBase, env), apimodel.NexmoApiSecretKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : complete.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : complete.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : complete.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : complete.go : firehose client was successfully initialized")

	awsCWClient = cloudwatch.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : complete.go : cloudwatch client was successfully initialized")

	baseCloudWatchNamespace, ok = os.LookupEnv("BASE_CLOUD_WATCH_NAMESPACE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : complete.go : env can not be empty BASE_CLOUD_WATCH_NAMESPACE")
	}
	anlogger.Debugf(nil, "lambda-initialization : complete.go : start with BASE_CLOUD_WATCH_NAMESPACE = [%s]", baseCloudWatchNamespace)

	nexmoCompleteMetricName, ok = os.LookupEnv("CLOUD_WATCH_NEXMO_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : complete.go : env can not be empty CLOUD_WATCH_NEXMO_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME")
	}
	anlogger.Debugf(nil, "lambda-initialization : complete.go : start with CLOUD_WATCH_NEXMO_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME = [%s]", nexmoCompleteMetricName)

	twilioCompleteMetricName, ok = os.LookupEnv("CLOUD_WATCH_TWILIO_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : complete.go : env can not be empty CLOUD_WATCH_TWILIO_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME")
	}
	anlogger.Debugf(nil, "lambda-initialization : complete.go : start with CLOUD_WATCH_TWILIO_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME = [%s]", twilioCompleteMetricName)

	newUserWasCreatedMetricName, ok = os.LookupEnv("CLOUD_WATCH_NEW_USER_WAS_CREATED")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : complete.go : env can not be empty CLOUD_WATCH_NEW_USER_WAS_CREATED")
	}
	anlogger.Debugf(nil, "lambda-initialization : complete.go : start with CLOUD_WATCH_NEW_USER_WAS_CREATED = [%s]", newUserWasCreatedMetricName)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "complete.go : handle request %v", request)

	if apimodel.IsItWarmUpRequest(request.Body, anlogger, lc) {
		return events.APIGatewayProxyResponse{}, nil
	}

	appVersion, isItAndroid, ok, errStr := apimodel.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "complete.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	switch isItAndroid {
	case true:
		if appVersion < apimodel.MinimalAndroidBuildNum {
			errStr := apimodel.TooOldAppVersionClientError
			anlogger.Warnf(lc, "complete.go : warning, too old Android version [%d] when min version is [%d]", appVersion, apimodel.MinimalAndroidBuildNum)
			anlogger.Errorf(lc, "complete.go : return %s to client", errStr)
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
		}
	case false:
		if appVersion < apimodel.MinimaliOSBuildNum {
			errStr := apimodel.TooOldAppVersionClientError
			anlogger.Warnf(lc, "complete.go : warning, too old iOS version [%d] when min version is [%d]", appVersion, apimodel.MinimaliOSBuildNum)
			anlogger.Errorf(lc, "complete.go : return %s to client", errStr)
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
		}
	}

	reqParam, ok := parseParams(request.Body, lc)
	if !ok {
		errStr := apimodel.WrongRequestParamsClientError
		anlogger.Errorf(lc, "complete.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	userInfo, ok, errStr := fetchBySessionId(reqParam.SessionId, lc)
	if !ok {
		anlogger.Errorf(lc, "complete.go : userId [%s], return %s to client", userInfo.UserId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	timeToComplete := time.Now().Unix() - userInfo.VerificationStartAt
	switch userInfo.VerifyProvider {
	case apimodel.Twilio:
		ok, errStr = apimodel.CompleteTwilioVerify(userInfo, reqParam.VerificationCode, twilioKey, anlogger, lc)
		if !ok {
			anlogger.Errorf(lc, "complete.go : userId [%s], return %s to client", userInfo.UserId, errStr)
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
		}
		apimodel.SendCloudWatchMetric(baseCloudWatchNamespace, twilioCompleteMetricName, int(timeToComplete), awsCWClient, anlogger, lc)
	case apimodel.Nexmo:
		ok, errStr = apimodel.CompleteNexmoVerify(userInfo, reqParam.VerificationCode, nexmoApiKey, nexmoApiSecret, anlogger, lc)
		if !ok {
			anlogger.Errorf(lc, "complete.go : userId [%s], return %s to client", userInfo.UserId, errStr)
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
		}
		apimodel.SendCloudWatchMetric(baseCloudWatchNamespace, nexmoCompleteMetricName, int(timeToComplete), awsCWClient, anlogger, lc)
	default:
		errorStr := apimodel.InternalServerError
		anlogger.Errorf(lc, "complete.go : unsupported verify provider [%s]", userInfo.VerifyProvider)
		anlogger.Errorf(lc, "complete.go : return %s to client", errorStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errorStr}, nil
	}

	event := apimodel.NewUserVerificationCompleteEvent(userInfo.UserId, userInfo.VerifyProvider, userInfo.Locale, userInfo.CountryCode, userInfo.VerificationStartAt)
	apimodel.SendAnalyticEvent(event, userInfo.UserId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	//ignore the errors
	updateVerifyStatusToComplete(userInfo.Phone, userInfo.UserId, lc)

	sessionToken, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "complete.go : error while generate sessionToken for userId [%s] : %v", userInfo.UserId, err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}

	userExist, ok, errStr := updateSessionToken(userInfo.UserId, sessionToken.String(), userInfo.Locale, lc)
	if !ok {
		anlogger.Errorf(lc, "complete.go : userId [%s], return %s to client", userInfo.UserId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		apimodel.AccessTokenUserIdClaim:       userInfo.UserId,
		apimodel.AccessTokenSessionTokenClaim: sessionToken.String(),
	})

	tokenToString, err := accessToken.SignedString([]byte(secretWord))
	if err != nil {
		errStr = apimodel.InternalServerError
		anlogger.Errorf(lc, "complete.go : error sign the token for userId [%s], return %s to the client : %v", errStr, err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	resp := apimodel.VerifyResp{AccessToken: tokenToString, AccountAlreadyExist: userExist}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "complete.go : error while marshaling resp object %v for userId [%s] : %v", resp, userInfo.UserId, err)
		anlogger.Errorf(lc, "complete.go : userId [%s], return %s to client", userInfo.UserId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}

	if !userExist {
		anlogger.Infof(lc, "complete.go : new user profile was created, userId [%s]", userInfo.UserId)
		apimodel.SendCloudWatchMetric(baseCloudWatchNamespace, newUserWasCreatedMetricName, 1, awsCWClient, anlogger, lc)
	}

	//return OK with AccessToken
	anlogger.Debugf(lc, "complete.go : return body=%s to client, userId [%s]", string(body), userInfo.UserId)
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

//return is everything ok and error string
func updateVerifyStatusToComplete(phone, userId string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "complete.go : update verify status to complete for phone [%s] and userId [%s]", phone, userId)

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#verifyStatus": aws.String(apimodel.VerificationStatusColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":verifyStatusV": {
					S: aws.String("complete"),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				apimodel.PhoneColumnName: {
					S: aws.String(phone),
				},
			},
			TableName:        aws.String(userTableName),
			UpdateExpression: aws.String("SET #verifyStatus = :verifyStatusV"),
		}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "complete.go : error while update verify status to complete for phone [%s] and userId [%s] : %v", phone, userId, err)
		return false, apimodel.InternalServerError
	}

	anlogger.Debugf(lc, "complete.go : successfully update verify status to complete for phone [%s] and userId [%s]", phone, userId)

	return true, ""
}

//return do we already have such user, ok, errorString if not ok
func updateSessionToken(userId, sessionToken, locale string, lc *lambdacontext.LambdaContext) (bool, bool, string) {
	anlogger.Debugf(lc, "complete.go : update sessionToken [%s], locale [%s] for userId [%s]", sessionToken, locale, userId)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#token":     aws.String(apimodel.SessionTokenColumnName),
			"#updatedAt": aws.String(apimodel.TokenUpdatedTimeColumnName),
			"#locale":    aws.String(apimodel.LocaleColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":tV": {
				S: aws.String(sessionToken),
			},
			":uV": {
				S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
			},
			":localeV": {
				S: aws.String(locale),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			apimodel.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		ReturnValues:     aws.String("ALL_OLD"),
		TableName:        aws.String(userProfileTable),
		UpdateExpression: aws.String("SET #token = :tV, #updatedAt = :uV, #locale = :localeV"),
	}

	result, err := awsDbClient.UpdateItem(input)

	if err != nil {
		anlogger.Errorf(lc, "complete.go : error update sessionToken [%s] and locale [%s] for userId [%s] : %v", sessionToken, locale, userId, err)
		return false, false, apimodel.InternalServerError
	}

	//if table already contains sex column for this userId it means that we already had this user
	_, ok := result.Attributes[apimodel.SexColumnName]

	anlogger.Debugf(lc, "complete.go : successfully update sessionToken [%s] and locale [%s] for userId [%s]", sessionToken, locale, userId)
	return ok, true, ""
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

	var provider string
	if providerAttr, ok := res.Items[0][apimodel.VerifyProviderColumnName]; ok {
		provider = *providerAttr.S
	}

	var requestId string
	if requestIdAttr, ok := res.Items[0][apimodel.VerifyRequestIdColumnName]; ok {
		requestId = *requestIdAttr.S
	}

	var verificationStartAt int64
	if verificationStartAtAttr, ok := res.Items[0][apimodel.VerificationStartAtColumnName]; ok {
		if value, err := strconv.ParseInt(*verificationStartAtAttr.N, 0, 64); err != nil {
			anlogger.Warnf(lc, "complete.go : error while verification start value [%s] , sessionId [%s] : %v", *verificationStartAtAttr.N, sessionId, err)
		} else {
			verificationStartAt = value
		}
	}

	var locale string
	if localeAttr, ok := res.Items[0][apimodel.LocaleColumnName]; ok {
		locale = *localeAttr.S
	}
	userInfo := &apimodel.UserInfo{
		UserId:              userId,
		SessionId:           sessId,
		Phone:               phone,
		CountryCode:         countryCode,
		PhoneNumber:         phonenumber,
		CustomerId:          customerId,
		VerifyProvider:      provider,
		VerifyRequestId:     requestId,
		VerificationStartAt: verificationStartAt,
		Locale:              locale,
	}

	anlogger.Debugf(lc, "complete.go : successfully fetch userInfo %v by sessionId [%s]", userInfo, sessionId)

	return userInfo, true, ""
}

func main() {
	basicLambda.Start(handler)
}
