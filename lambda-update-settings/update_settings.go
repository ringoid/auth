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
	"github.com/aws/aws-sdk-go/aws/awserr"
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
		fmt.Printf("update_settings.go : env can not be empty ENV")
		os.Exit(1)
	}
	fmt.Printf("update_settings.go : start with ENV = [%s]", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("update_settings.go : env can not be empty PAPERTRAIL_LOG_ADDRESS")
		os.Exit(1)
	}
	fmt.Printf("update_settings.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "update-settings-auth"))
	if err != nil {
		fmt.Errorf("update_settings.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "update_settings.go : logger was successfully initialized")

	neo4jurl, ok = os.LookupEnv("NEO4J_URL")
	if !ok {
		fmt.Printf("update_settings.go : env can not be empty NEO4J_URL")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "update_settings.go : start with NEO4J_URL = [%s]", neo4jurl)

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("update_settings.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "update_settings.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	userSettingsTable, ok = os.LookupEnv("USER_SETTINGS_TABLE")
	if !ok {
		fmt.Printf("update_settings.go : env can not be empty USER_SETTINGS_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "update_settings.go : start with USER_SETTINGS_TABLE = [%s]", userSettingsTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "update_settings.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "update_settings.go : aws session was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "update_settings.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "update_settings.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "update_settings.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "update_settings.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "update_settings.go : start handle request %v", request)

	reqParam, ok, errStr := parseParams(request.Body, lc)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	userId, ok, errStr := apimodel.FindUserId(reqParam.AccessToken, userProfileTable, awsDbClient, anlogger, lc)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	settings := apimodel.NewUserSettings(userId, reqParam)

	ok, errStr = updateUserSettings(settings, lc)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	event := apimodel.NewUserSettingsUpdatedEvent(settings)
	apimodel.SendAnalyticEvent(event, settings.UserId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	resp := apimodel.BaseResponse{}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "update_settings.go : error while marshaling resp object for userId [%s] : %v", userId, err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	anlogger.Debugf(lc, "update_settings.go : return body resp [%s] for userId [%s]", string(body), userId)
	//return OK with AccessToken
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.UpdateSettingsReq, bool, string) {
	anlogger.Debugf(lc, "update_settings.go : start parsing request body [%s]", params)
	var req apimodel.UpdateSettingsReq
	err := json.Unmarshal([]byte(params), &req)
	if err != nil {
		anlogger.Errorf(lc, "update_settings.go : error marshaling required params from the string [%s] : %v", params, err)
		return nil, false, apimodel.InternalServerError
	}

	if req.WhoCanSeePhoto != "OPPOSITE" && req.WhoCanSeePhoto != "INCOGNITO" && req.WhoCanSeePhoto != "ONLY_ME" {
		anlogger.Errorf(lc, "update_settings.go : wrong whoCanSeePhoto [%s] request param, req %v", req.WhoCanSeePhoto, req)
		return nil, false, apimodel.WrongRequestParamsClientError
	}

	if req.SafeDistanceInMeter < 0 {
		anlogger.Errorf(lc, "update_settings.go : wrong safeDistanceInMeter [%d] request param, req %v", req.SafeDistanceInMeter, req)
		return nil, false, apimodel.WrongRequestParamsClientError
	}

	if req.PushLikes != "NONE" && req.PushLikes != "EVERY" && req.PushLikes != "10_NEW" && req.PushLikes != "100_NEW" {
		anlogger.Errorf(lc, "update_settings.go : wrong pushLikes [%s] request param, req %v", req.PushLikes, req)
		return nil, false, apimodel.WrongRequestParamsClientError
	}

	if req.InAppLikes != "NONE" && req.InAppLikes != "EVERY" && req.InAppLikes != "10_NEW" && req.InAppLikes != "100_NEW" {
		anlogger.Errorf(lc, "update_settings.go : wrong inAppLikes [%s] request param, req %v", req.InAppLikes, req)
		return nil, false, apimodel.WrongRequestParamsClientError
	}

	anlogger.Debugf(lc, "update_settings.go : successfully parse request string [%s] to [%v]", params, req)
	return &req, true, ""
}

//return ok and error string
func updateUserSettings(settings *apimodel.UserSettings, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "update_settings.go : start update user settings in Dynamo for userId [%s]", settings.UserId)
	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#whoCanSeePhoto":      aws.String(apimodel.WhoCanSeePhotoColumnName),
				"#safeDistanceInMeter": aws.String(apimodel.SafeDistanceInMeterColumnName),
				"#pushMessages":        aws.String(apimodel.PushMessagesColumnName),
				"#pushMatches":         aws.String(apimodel.PushMatchesColumnName),
				"#pushLikes":           aws.String(apimodel.PushLikesColumnName),
				"#inAppMessages":       aws.String(apimodel.InAppMessagesColumnName),
				"#inAppMatches":        aws.String(apimodel.InAppMatchesColumnName),
				"#inAppLikes":          aws.String(apimodel.InAppLikesColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":whoCanSeePhotoV": {
					S: aws.String(settings.WhoCanSeePhoto),
				},
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
				":inAppMessagesV": {
					BOOL: aws.Bool(settings.InAppMessages),
				},
				":inAppMatchesV": {
					BOOL: aws.Bool(settings.InAppMatches),
				},
				":inAppLikesV": {
					S: aws.String(settings.InAppLikes),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				apimodel.UserIdColumnName: {
					S: aws.String(settings.UserId),
				},
			},

			TableName:        aws.String(userSettingsTable),
			UpdateExpression: aws.String("SET #whoCanSeePhoto = :whoCanSeePhotoV, #safeDistanceInMeter = :safeDistanceInMeterV, #pushMessages = :pushMessagesV, #pushMatches = :pushMatchesV, #pushLikes = :pushLikesV, #inAppMessages = :inAppMessagesV, #inAppMatches = :inAppMatchesV, #inAppLikes = :inAppLikesV"),
		}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				anlogger.Warnf(lc, "start.go : warning, settings for userId [%s] already exist", settings.UserId)
				return true, ""
			}
		}
		anlogger.Errorf(lc, "start.go : error while update user settings for userId [%s] : %v", settings.UserId, err)
		return false, apimodel.InternalServerError
	}

	anlogger.Debugf(lc, "update_settings.go : successfully update user settings in Dynamo for userId [%s]", settings.UserId)
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}