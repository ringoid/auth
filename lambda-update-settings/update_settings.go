package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"../apimodel"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"strconv"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/ringoid/commons"
	"strings"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var userSettingsTable string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string
var secretWord string
var commonStreamName string
var awsKinesisClient *kinesis.Kinesis

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : update_settings.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : update_settings.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : update_settings.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : update_settings.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "update-settings-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : update_settings.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_settings.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : update_settings.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_settings.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	userSettingsTable, ok = os.LookupEnv("USER_SETTINGS_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : update_settings.go : env can not be empty USER_SETTINGS_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_settings.go : start with USER_SETTINGS_TABLE = [%s]", userSettingsTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : update_settings.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_settings.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : update_settings.go : dynamodb client was successfully initialized")

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : update_settings.go : env can not be empty COMMON_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_settings.go : start with DELIVERY_STREAM = [%s]", commonStreamName)

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : update_settings.go : kinesis client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : update_settings.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : update_settings.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : update_settings.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	userAgent := request.Headers["user-agent"]
	if strings.HasPrefix(userAgent, "ELB-HealthChecker") {
		return commons.NewServiceResponse("{}"), nil
	}

	if request.HTTPMethod != "POST" {
		return commons.NewWrongHttpMethodServiceResponse(), nil
	}
	sourceIp := request.Headers["x-forwarded-for"]

	anlogger.Debugf(lc, "update_settings.go : start handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	reqParam, ok, errStr := parseParams(request.Body, lc)
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	userId, _, _, ok, errStr := commons.Login(appVersion, isItAndroid, reqParam.AccessToken, secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	settings := apimodel.NewUserSettings(userId, reqParam)

	ok, errStr = updateUserSettings(settings, lc)
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : userId [%s], return %s to client", userId, errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	event := commons.NewUserSettingsUpdatedEvent(userId, sourceIp, settings.SafeDistanceInMeter, settings.PushMessages, settings.PushMatches, settings.PushLikes)
	commons.SendAnalyticEvent(event, settings.UserId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	partitionKey := userId
	ok, errStr = commons.SendCommonEvent(event, userId, commonStreamName, partitionKey, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : userId [%s], return %s to client", userId, errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	resp := commons.BaseResponse{}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "update_settings.go : error while marshaling resp object for userId [%s] : %v", userId, err)
		return commons.NewServiceResponse(commons.InternalServerError), nil
	}
	anlogger.Debugf(lc, "update_settings.go : return body=%s for userId [%s]", string(body), userId)
	//return OK with AccessToken
	return commons.NewServiceResponse(string(body)), nil
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.UpdateSettingsReq, bool, string) {
	anlogger.Debugf(lc, "update_settings.go : parse request body [%s]", params)
	var req apimodel.UpdateSettingsReq
	err := json.Unmarshal([]byte(params), &req)
	if err != nil {
		anlogger.Errorf(lc, "update_settings.go : error marshaling required params from the string [%s] : %v", params, err)
		return nil, false, commons.InternalServerError
	}

	if req.AccessToken == "" {
		anlogger.Errorf(lc, "update_settings.go : empty accessToken request param, req %v", req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	if req.SafeDistanceInMeter < 0 {
		anlogger.Errorf(lc, "update_settings.go : wrong safeDistanceInMeter [%d] request param, req %v", req.SafeDistanceInMeter, req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	if req.PushLikes != "NONE" && req.PushLikes != "EVERY" && req.PushLikes != "10_NEW" && req.PushLikes != "100_NEW" {
		anlogger.Errorf(lc, "update_settings.go : wrong pushLikes [%s] request param, req %v", req.PushLikes, req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	anlogger.Debugf(lc, "update_settings.go : successfully parse request string [%s] to %v", params, req)
	return &req, true, ""
}

//return ok and error string
func updateUserSettings(settings *apimodel.UserSettings, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "update_settings.go : start update user settings for userId [%s], settings=%v", settings.UserId, settings)
	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#safeDistanceInMeter": aws.String(commons.SafeDistanceInMeterColumnName),
				"#pushMessages":        aws.String(commons.PushMessagesColumnName),
				"#pushMatches":         aws.String(commons.PushMatchesColumnName),
				"#pushLikes":           aws.String(commons.PushLikesColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":safeDistanceInMeterV": {
					N: aws.String(strconv.Itoa(settings.SafeDistanceInMeter)),
				},
				":pushMessagesV": {
					BOOL: aws.Bool(settings.PushMessages),
				},
				":pushMatchesV": {
					BOOL: aws.Bool(settings.PushMatches),
				},
				":pushLikesV": {
					S: aws.String(settings.PushLikes),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				commons.UserIdColumnName: {
					S: aws.String(settings.UserId),
				},
			},

			TableName:        aws.String(userSettingsTable),
			UpdateExpression: aws.String("SET #safeDistanceInMeter = :safeDistanceInMeterV, #pushMessages = :pushMessagesV, #pushMatches = :pushMatchesV, #pushLikes = :pushLikesV"),
		}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "update_settings.go : error update user settings for userId [%s], settings=%v : %v", settings.UserId, settings, err)
		return false, commons.InternalServerError
	}

	anlogger.Infof(lc, "update_settings.go : successfully update user settings for userId [%s], settings=%v", settings.UserId, settings)
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
