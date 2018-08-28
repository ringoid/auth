package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"../sys_log"
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
)

var anlogger *syslog.Logger
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var userSettingsTable string
var neo4jurl string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("get_settings.go.go : env can not be empty ENV")
		os.Exit(1)
	}
	fmt.Printf("get_settings.go.go : start with ENV = [%s]", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("get_settings.go.go : env can not be empty PAPERTRAIL_LOG_ADDRESS")
		os.Exit(1)
	}
	fmt.Printf("get_settings.go.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "update-settings-auth"))
	if err != nil {
		fmt.Errorf("get_settings.go.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "get_settings.go.go : logger was successfully initialized")

	neo4jurl, ok = os.LookupEnv("NEO4J_URL")
	if !ok {
		fmt.Printf("get_settings.go.go : env can not be empty NEO4J_URL")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "get_settings.go.go : start with NEO4J_URL = [%s]", neo4jurl)

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("get_settings.go.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "get_settings.go.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	userSettingsTable, ok = os.LookupEnv("USER_SETTINGS_TABLE")
	if !ok {
		fmt.Printf("get_settings.go.go : env can not be empty USER_SETTINGS_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "get_settings.go.go : start with USER_SETTINGS_TABLE = [%s]", userSettingsTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "get_settings.go.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "get_settings.go.go : aws session was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "get_settings.go.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "get_settings.go.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "get_settings.go.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "get_settings.go.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "get_settings.go.go : start handle request %v", request)
	anlogger.Debugf(lc, "get_settings.go.go : query string params %v", request.QueryStringParameters)

	accessToken := request.QueryStringParameters["accessToken"]

	userId, ok, errStr := apimodel.FindUserId(accessToken, userProfileTable, awsDbClient, anlogger, lc)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	settings, ok, errStr := getUserSettings(userId, lc)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	resp := apimodel.GetSettingsResp{
		WhoCanSeePhoto:      settings.WhoCanSeePhoto,
		SafeDistanceInMeter: settings.SafeDistanceInMeter,
		PushMessages:        settings.PushMessages,
		PushMatches:         settings.PushMatches,
		PushLikes:           settings.PushLikes,
		InAppMessages:       settings.InAppMessages,
		InAppMatches:        settings.InAppMatches,
		InAppLikes:          settings.InAppLikes,
	}

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "get_settings.go.go : error while marshaling resp object for userId [%s] : %v", userId, err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	anlogger.Debugf(lc, "get_settings.go.go : return body resp [%s] for userId [%s]", string(body), userId)
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

//return userSettings, ok and error string
func getUserSettings(userId string, lc *lambdacontext.LambdaContext) (*apimodel.UserSettings, bool, string) {
	anlogger.Debugf(lc, "get_settings.go : start get user settings from Dynamo for userId [%s]", userId)
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			apimodel.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		TableName: aws.String(userSettingsTable),
	}

	result, err := awsDbClient.GetItem(input)
	if err != nil {
		anlogger.Errorf(lc, "get_settings.go : error during get user settings for userId [%s] : %v", userId, err)
		return nil, false, apimodel.InternalServerError
	}

	if len(result.Item) == 0 {
		anlogger.Errorf(lc, "get_settings.go : empty settings for userId [%s]", userId)
		return nil, false, apimodel.InternalServerError
	}

	safeD, err := strconv.Atoi(*result.Item[apimodel.SafeDistanceInMeterColumnName].N)
	if err != nil {
		anlogger.Errorf(lc, "get_settings.go : error while parsing db response for userId [%s] : %v", userId, err)
		return nil, false, apimodel.InternalServerError
	}

	userSettings := &apimodel.UserSettings{
		UserId:              *result.Item[apimodel.UserIdColumnName].S,
		WhoCanSeePhoto:      *result.Item[apimodel.WhoCanSeePhotoColumnName].S,
		SafeDistanceInMeter: safeD,
		PushMessages:        *result.Item[apimodel.PushMessagesColumnName].BOOL,
		PushMatches:         *result.Item[apimodel.PushMatchesColumnName].BOOL,
		PushLikes:           *result.Item[apimodel.PushLikesColumnName].S,
		InAppMessages:       *result.Item[apimodel.InAppMessagesColumnName].BOOL,
		InAppMatches:        *result.Item[apimodel.InAppMatchesColumnName].BOOL,
		InAppLikes:          *result.Item[apimodel.InAppLikesColumnName].S,
	}
	anlogger.Debugf(lc, "get_settings.go : successfully return user setting for userId  [%s], setting=%v", userId, userSettings)
	return userSettings, true, ""
}

func main() {
	basicLambda.Start(handler)
}