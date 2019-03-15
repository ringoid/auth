package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/ringoid/commons"
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
		fmt.Printf("lambda-initialization : get_settings.go.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : get_settings.go.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : get_settings.go.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : get_settings.go.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "get-settings-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : get_settings.go.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_settings.go.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("lambda-initialization : get_settings.go.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_settings.go.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	userSettingsTable, ok = os.LookupEnv("USER_SETTINGS_TABLE")
	if !ok {
		fmt.Printf("lambda-initialization : get_settings.go.go : env can not be empty USER_SETTINGS_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_settings.go.go : start with USER_SETTINGS_TABLE = [%s]", userSettingsTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : get_settings.go.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_settings.go.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : get_settings.go.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : get_settings.go.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_settings.go.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : get_settings.go.go : firehose client was successfully initialized")

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : get_settings.go : env can not be empty COMMON_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : get_settings.go : start with COMMON_STREAM = [%s]", commonStreamName)

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : get_settings.go : kinesis client was successfully initialized")
}

func handler(ctx context.Context, request events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	userAgent := request.Headers["user-agent"]
	if strings.HasPrefix(userAgent, "ELB-HealthChecker") {
		return commons.NewServiceResponse("{}"), nil
	}

	if request.HTTPMethod != "GET" {
		return commons.NewWrongHttpMethodServiceResponse(), nil
	}
	sourceIp := request.Headers["x-forwarded-for"]

	anlogger.Debugf(lc, "get_settings.go : start handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "get_settings.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	accessToken, ok := request.QueryStringParameters["accessToken"]
	if !ok {
		errStr := commons.WrongRequestParamsClientError
		anlogger.Errorf(lc, "get_settings.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	userId, _, _, ok, errStr := commons.Login(appVersion, isItAndroid, accessToken, secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "get_settings.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	settings, ok, errStr := getUserSettings(userId, lc)
	if !ok {
		anlogger.Errorf(lc, "get_settings.go : userId [%s], return %s to client", userId, errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	resp := apimodel.GetSettingsResp{
		SafeDistanceInMeter: settings.SafeDistanceInMeter,
		PushMessages:        settings.PushMessages,
		PushMatches:         settings.PushMatches,
		PushLikes:           settings.PushLikes,
	}

	event := commons.NewGetUserSettingsEvent(userId, sourceIp)
	commons.SendAnalyticEvent(event, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "get_settings.go : error while marshaling resp object for userId [%s], resp=%v : %v", userId, resp, err)
		return commons.NewServiceResponse(commons.InternalServerError), nil
	}
	anlogger.Debugf(lc, "get_settings.go : return body=%s to the client, userId [%s]", string(body), userId)
	return commons.NewServiceResponse(string(body)), nil
}

//return userSettings, ok and error string
func getUserSettings(userId string, lc *lambdacontext.LambdaContext) (*apimodel.UserSettings, bool, string) {
	anlogger.Debugf(lc, "get_settings.go : get user settings for userId [%s]", userId)
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(userSettingsTable),
	}

	result, err := awsDbClient.GetItem(input)
	if err != nil {
		anlogger.Errorf(lc, "get_settings.go : error get user settings for userId [%s] : %v", userId, err)
		return nil, false, commons.InternalServerError
	}

	if len(result.Item) == 0 {
		anlogger.Errorf(lc, "get_settings.go : empty settings for userId [%s]", userId)
		return nil, false, commons.InternalServerError
	}

	safeD, err := strconv.Atoi(*result.Item[commons.SafeDistanceInMeterColumnName].N)
	if err != nil {
		anlogger.Errorf(lc, "get_settings.go : error while parsing db response for userId [%s], resp=%v : %v", userId, result.Item, err)
		return nil, false, commons.InternalServerError
	}

	userSettings := &apimodel.UserSettings{
		UserId:              *result.Item[commons.UserIdColumnName].S,
		SafeDistanceInMeter: safeD,
		PushMessages:        *result.Item[commons.PushMessagesColumnName].BOOL,
		PushMatches:         *result.Item[commons.PushMatchesColumnName].BOOL,
		PushLikes:           *result.Item[commons.PushLikesColumnName].S,
	}
	anlogger.Infof(lc, "get_settings.go : successfully return user setting for userId [%s], setting=%v", userId, userSettings)
	return userSettings, true, ""
}

func main() {
	basicLambda.Start(handler)
}
