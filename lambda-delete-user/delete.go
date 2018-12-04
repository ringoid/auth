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
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/ringoid/commons"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

var anlogger *commons.Logger
var secretWord string
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var userSettingsTable string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string
var commonStreamName string
var internalStreamName string
var awsKinesisClient *kinesis.Kinesis

var baseCloudWatchNamespace string
var userDeleteHimselfMetricName string
var awsCWClient *cloudwatch.CloudWatch

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : delete.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : delete.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : delete.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : delete.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "delete-user-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : delete.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : delete.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : delete.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : delete.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	userSettingsTable, ok = os.LookupEnv("USER_SETTINGS_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : delete.go : env can not be empty USER_SETTINGS_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : delete.go : start with USER_SETTINGS_TABLE = [%s]", userSettingsTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : delete.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : delete.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : delete.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : delete.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : delete.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : delete.go : firehose client was successfully initialized")

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : delete.go : env can not be empty COMMON_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : delete.go : start with COMMON_STREAM = [%s]", commonStreamName)

	internalStreamName, ok = os.LookupEnv("INTERNAL_STREAM_NAME")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : delete.go : env can not be empty INTERNAL_STREAM_NAME")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : delete.go : start with INTERNAL_STREAM_NAME = [%s]", internalStreamName)

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : delete.go : kinesis client was successfully initialized")

	baseCloudWatchNamespace, ok = os.LookupEnv("BASE_CLOUD_WATCH_NAMESPACE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : delete.go : env can not be empty BASE_CLOUD_WATCH_NAMESPACE")
	}
	anlogger.Debugf(nil, "lambda-initialization : delete.go : start with BASE_CLOUD_WATCH_NAMESPACE = [%s]", baseCloudWatchNamespace)

	userDeleteHimselfMetricName, ok = os.LookupEnv("CLOUD_WATCH_USER_DELETE_HIMSELF")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : create.go : env can not be empty CLOUD_WATCH_USER_DELETE_HIMSELF")
	}
	anlogger.Debugf(nil, "lambda-initialization : create.go : start with CLOUD_WATCH_USER_DELETE_HIMSELF = [%s]", userDeleteHimselfMetricName)

	awsCWClient = cloudwatch.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : delete.go : cloudwatch client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "delete.go : handle request %v", request)

	if commons.IsItWarmUpRequest(request.Body, anlogger, lc) {
		return events.APIGatewayProxyResponse{}, nil
	}

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "delete.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	reqParam, ok := parseParams(request.Body, lc)
	if !ok {
		errStr := commons.WrongRequestParamsClientError
		anlogger.Errorf(lc, "delete.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	userId, _, userReportStatus, ok, errStr := commons.Login(appVersion, isItAndroid, reqParam.AccessToken, secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "delete.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	event := commons.NewUserCallDeleteHimselfEvent(userId, userReportStatus)
	commons.SendAnalyticEvent(event, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	//send common events for neo4j
	partitionKey := userId
	ok, errStr = commons.SendCommonEvent(event, userId, commonStreamName, partitionKey, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "create.go : userId [%s], return %s to client", userId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	//send event to internal stream
	ok, errStr = commons.SendCommonEvent(event, userId, internalStreamName, partitionKey, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "create.go : userId [%s], return %s to client", userId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	//send cloudwatch metric
	commons.SendCloudWatchMetric(baseCloudWatchNamespace, userDeleteHimselfMetricName, 1, awsCWClient, anlogger, lc)

	if userReportStatus == commons.UserWasReportedStatus || userReportStatus == commons.UserWasReportInitiatorStatus {
		anlogger.Infof(lc, "delete.go : user with userId [%s] was reported or was report initiator, so don't delete him", userId)
		ok, errStr = apimodel.DisableCurrentAccessToken(userId, userProfileTable, awsDbClient, anlogger, lc)
		if !ok {
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
		}
	} else {
		ok, errStr = apimodel.DeleteUserFromAuthService(userId, userProfileTable, userSettingsTable, awsDbClient, anlogger, lc)
		if !ok {
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
		}
	}

	resp := commons.BaseResponse{}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "delete.go : error while marshaling resp object %v for userId [%s] : %v", resp, userId, err)
		anlogger.Errorf(lc, "delete.go : userId [%s], return %s to client", userId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: commons.InternalServerError}, nil
	}
	anlogger.Debugf(lc, "delete.go : return body=%s to client, userId [%s]", string(body), userId)
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.DeleteReq, bool) {
	var req apimodel.DeleteReq
	err := json.Unmarshal([]byte(params), &req)

	if err != nil {
		anlogger.Errorf(lc, "delete.go : error unmarshal required params from the string %s : %v", params, err)
		return nil, false
	}

	if req.AccessToken == "" {
		anlogger.Errorf(lc, "delete.go : one of the required param is nil or empty, req %v", req)
		return nil, false
	}

	return &req, true
}

func main() {
	basicLambda.Start(handler)
}
