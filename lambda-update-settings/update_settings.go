package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/ringoid/commons"
	"strings"
	"../apimodel"
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

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "update-settings-auth"), apimodel.IsDebugLogEnabled)
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

	reqParamMap, ok, errStr := parseParams(request.Body, lc)
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	userId, _, _, ok, errStr := commons.Login(appVersion, isItAndroid, reqParamMap["accessToken"].(string), secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = updateUserSettings(userId, reqParamMap, lc)
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : userId [%s], return %s to client", userId, errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	localeIntr, localeOk := reqParamMap["locale"]
	var localeStr string
	if localeOk {
		localeStr = localeIntr.(string)
	}
	pushIntr, pushOk := reqParamMap["push"]
	var pushBool bool
	if pushOk {
		pushBool = pushIntr.(bool)
	}

	pushNewLikeIntr, pushNewLikeOk := reqParamMap["pushNewLike"]
	var pushNewLikeBool bool
	if pushNewLikeOk {
		pushNewLikeBool = pushNewLikeIntr.(bool)
	}

	pushNewMessageIntr, pushNewMessageOk := reqParamMap["pushNewMessage"]
	var pushNewMessageBool bool
	if pushNewMessageOk {
		pushNewMessageBool = pushNewMessageIntr.(bool)
	}

	pushNewMatchIntr, pushNewMatchOk := reqParamMap["pushNewMatch"]
	var pushNewMatchBool bool
	if pushNewMatchOk {
		pushNewMatchBool = pushNewMatchIntr.(bool)
	}

	pushVibrationIntr, pushVibrationOk := reqParamMap["vibration"]
	var pushVibrationBool bool
	if pushVibrationOk {
		pushVibrationBool = pushVibrationIntr.(bool)
	}

	timeZoneFlt, timeZoneOk := reqParamMap["timeZone"]
	var timeZoneInt int
	if timeZoneOk {
		timeZoneInt = int(timeZoneFlt.(float64))
	}
	event :=
		commons.NewUserSettingsUpdatedEvent(userId, sourceIp, localeStr, localeOk,
			pushBool, pushNewLikeBool, pushNewMatchBool, pushNewMessageBool,
			pushOk, pushNewLikeOk, pushNewMatchOk, pushNewMessageOk,
			pushVibrationBool, pushVibrationOk,
			timeZoneInt, timeZoneOk)
	commons.SendAnalyticEvent(event, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

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

func parseParams(params string, lc *lambdacontext.LambdaContext) (map[string]interface{}, bool, string) {
	anlogger.Debugf(lc, "update_settings.go : parse request body [%s]", params)
	var reqMap map[string]interface{}
	err := json.Unmarshal([]byte(params), &reqMap)
	if err != nil {
		anlogger.Errorf(lc, "update_settings.go : error marshaling required params from the string [%s] : %v", params, err)
		return nil, false, commons.InternalServerError
	}

	accessTokenInter, ok := reqMap["accessToken"]
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : empty or nil accessToken request param, req %v", reqMap)
		return nil, false, commons.WrongRequestParamsClientError
	}
	accessToken, ok := accessTokenInter.(string)
	if !ok || accessToken == "" {
		anlogger.Errorf(lc, "update_settings.go : wrong format or empty accessToken request param, req %v", reqMap)
		return nil, false, commons.WrongRequestParamsClientError
	}

	pushIntr, ok := reqMap["push"]
	if ok {
		_, ok = pushIntr.(bool)
		if !ok {
			anlogger.Errorf(lc, "update_settings.go : error format of push in request param, req %v", reqMap)
			return nil, false, commons.WrongRequestParamsClientError
		}
	}

	pushNewLikeIntr, ok := reqMap["pushNewLike"]
	if ok {
		_, ok = pushNewLikeIntr.(bool)
		if !ok {
			anlogger.Errorf(lc, "update_settings.go : error format of pushNewLike in request param, req %v", reqMap)
			return nil, false, commons.WrongRequestParamsClientError
		}
	}

	pushNewMatchIntr, ok := reqMap["pushNewMatch"]
	if ok {
		_, ok = pushNewMatchIntr.(bool)
		if !ok {
			anlogger.Errorf(lc, "update_settings.go : error format of pushNewMatch in request param, req %v", reqMap)
			return nil, false, commons.WrongRequestParamsClientError
		}
	}

	pushNewMessageIntr, ok := reqMap["pushNewMessage"]
	if ok {
		_, ok = pushNewMessageIntr.(bool)
		if !ok {
			anlogger.Errorf(lc, "update_settings.go : error format of pushNewMessage in request param, req %v", reqMap)
			return nil, false, commons.WrongRequestParamsClientError
		}
	}

	timeZoneFlt, ok := reqMap["timeZone"]
	if ok {
		_, ok = timeZoneFlt.(float64)
		if !ok {
			anlogger.Errorf(lc, "update_settings.go : error format of timeZone in request param, req %v", reqMap)
			return nil, false, commons.WrongRequestParamsClientError
		}
	}

	pushVibrationIntr, ok := reqMap["vibration"]
	if ok {
		_, ok = pushVibrationIntr.(bool)
		if !ok {
			anlogger.Errorf(lc, "update_settings.go : error format of vibration in request param, req %v", reqMap)
			return nil, false, commons.WrongRequestParamsClientError
		}
	}

	anlogger.Debugf(lc, "update_settings.go : successfully parse request string [%s] to %v", params, reqMap)
	return reqMap, true, ""
}

//return ok and error string
func updateUserSettings(userId string, mapSettings map[string]interface{}, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "update_settings.go : start update user settings for userId [%s], settings=%v", userId, mapSettings)

	for key, value := range mapSettings {
		anlogger.Debugf(lc, "update_settings.go : update key [%v], value [%v]", key, value)
		if key == "locale" {
			input :=
				&dynamodb.UpdateItemInput{
					ExpressionAttributeNames: map[string]*string{
						"#locale": aws.String(commons.LocaleColumnName),
					},
					ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
						":localeV": {
							S: aws.String(value.(string)),
						},
					},
					Key: map[string]*dynamodb.AttributeValue{
						commons.UserIdColumnName: {
							S: aws.String(userId),
						},
					},

					TableName:        aws.String(userSettingsTable),
					UpdateExpression: aws.String("SET #locale = :localeV"),
				}

			_, err := awsDbClient.UpdateItem(input)
			if err != nil {
				anlogger.Errorf(lc, "update_settings.go : error update user locale settings for userId [%s], settings=%v : %v", userId, mapSettings, err)
				return false, commons.InternalServerError
			}
		} else if key == "timeZone" {
			input :=
				&dynamodb.UpdateItemInput{
					ExpressionAttributeNames: map[string]*string{
						"#timeZone": aws.String(commons.TimeZoneColumnName),
					},
					ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
						":timeZoneV": {
							N: aws.String(fmt.Sprintf("%v", value.(float64))),
						},
					},
					Key: map[string]*dynamodb.AttributeValue{
						commons.UserIdColumnName: {
							S: aws.String(userId),
						},
					},

					TableName:        aws.String(userSettingsTable),
					UpdateExpression: aws.String("SET #timeZone = :timeZoneV"),
				}

			_, err := awsDbClient.UpdateItem(input)
			if err != nil {
				anlogger.Errorf(lc, "update_settings.go : error update user timeZone settings for userId [%s], settings=%v : %v", userId, mapSettings, err)
				return false, commons.InternalServerError
			}
		} else if key == "push" {
			//we already checked that we can convert to bool in parse param
			input :=
				&dynamodb.UpdateItemInput{
					ExpressionAttributeNames: map[string]*string{
						"#push": aws.String(commons.PushColumnName),
					},
					ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
						":pushV": {
							BOOL: aws.Bool(value.(bool)),
						},
					},
					Key: map[string]*dynamodb.AttributeValue{
						commons.UserIdColumnName: {
							S: aws.String(userId),
						},
					},

					TableName:        aws.String(userSettingsTable),
					UpdateExpression: aws.String("SET #push = :pushV"),
				}

			_, err := awsDbClient.UpdateItem(input)
			if err != nil {
				anlogger.Errorf(lc, "update_settings.go : error update user push settings for userId [%s], settings=%v : %v", userId, mapSettings, err)
				return false, commons.InternalServerError
			}
		} else if key == "pushNewLike" {
			//we already checked that we can convert to bool in parse param
			input :=
				&dynamodb.UpdateItemInput{
					ExpressionAttributeNames: map[string]*string{
						"#pushNewLike": aws.String(commons.PushNewLikeColumnName),
					},
					ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
						":pushNewLikeV": {
							BOOL: aws.Bool(value.(bool)),
						},
					},
					Key: map[string]*dynamodb.AttributeValue{
						commons.UserIdColumnName: {
							S: aws.String(userId),
						},
					},

					TableName:        aws.String(userSettingsTable),
					UpdateExpression: aws.String("SET #pushNewLike = :pushNewLikeV"),
				}

			_, err := awsDbClient.UpdateItem(input)
			if err != nil {
				anlogger.Errorf(lc, "update_settings.go : error update user pushNewLike settings for userId [%s], settings=%v : %v", userId, mapSettings, err)
				return false, commons.InternalServerError
			}
		} else if key == "pushNewMatch" {
			//we already checked that we can convert to bool in parse param
			input :=
				&dynamodb.UpdateItemInput{
					ExpressionAttributeNames: map[string]*string{
						"#pushNewMatch": aws.String(commons.PushNewMatchColumnName),
					},
					ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
						":pushNewMatchV": {
							BOOL: aws.Bool(value.(bool)),
						},
					},
					Key: map[string]*dynamodb.AttributeValue{
						commons.UserIdColumnName: {
							S: aws.String(userId),
						},
					},

					TableName:        aws.String(userSettingsTable),
					UpdateExpression: aws.String("SET #pushNewMatch = :pushNewMatchV"),
				}

			_, err := awsDbClient.UpdateItem(input)
			if err != nil {
				anlogger.Errorf(lc, "update_settings.go : error update user pushNewMatch settings for userId [%s], settings=%v : %v", userId, mapSettings, err)
				return false, commons.InternalServerError
			}
		} else if key == "pushNewMessage" {
			//we already checked that we can convert to bool in parse param
			input :=
				&dynamodb.UpdateItemInput{
					ExpressionAttributeNames: map[string]*string{
						"#pushNewMessage": aws.String(commons.PushNewMessageColumnName),
					},
					ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
						":pushNewMessageV": {
							BOOL: aws.Bool(value.(bool)),
						},
					},
					Key: map[string]*dynamodb.AttributeValue{
						commons.UserIdColumnName: {
							S: aws.String(userId),
						},
					},

					TableName:        aws.String(userSettingsTable),
					UpdateExpression: aws.String("SET #pushNewMessage = :pushNewMessageV"),
				}

			_, err := awsDbClient.UpdateItem(input)
			if err != nil {
				anlogger.Errorf(lc, "update_settings.go : error update user pushNewMessage settings for userId [%s], settings=%v : %v", userId, mapSettings, err)
				return false, commons.InternalServerError
			}
		} else if key == "vibration" {
			//we already checked that we can convert to bool in parse param
			input :=
				&dynamodb.UpdateItemInput{
					ExpressionAttributeNames: map[string]*string{
						"#pushVibration": aws.String(commons.PushVibrationColumnName),
					},
					ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
						":pushVibrationV": {
							BOOL: aws.Bool(value.(bool)),
						},
					},
					Key: map[string]*dynamodb.AttributeValue{
						commons.UserIdColumnName: {
							S: aws.String(userId),
						},
					},

					TableName:        aws.String(userSettingsTable),
					UpdateExpression: aws.String("SET #pushVibration = :pushVibrationV"),
				}

			_, err := awsDbClient.UpdateItem(input)
			if err != nil {
				anlogger.Errorf(lc, "update_settings.go : error update user vibration settings for userId [%s], settings=%v : %v", userId, mapSettings, err)
				return false, commons.InternalServerError
			}
		}
	} //end for

	anlogger.Infof(lc, "update_settings.go : successfully update user settings for userId [%s], settings=%v", userId, mapSettings)
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
