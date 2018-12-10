package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"strings"
	"github.com/ringoid/commons"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
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
		fmt.Printf("lambda-initialization : internal_get_user_id.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : internal_get_user_id.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : internal_get_user_id.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : internal_get_user_id.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "internal-get-user-id-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization :  internal_get_user_id.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_get_user_id.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : internal_get_user_id.go : env can not be empty USER_PROFILE_TABLE")
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_get_user_id.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : internal_get_user_id.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_get_user_id.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : internal_get_user_id.go : dynamodb client was successfully initialized")

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : internal_get_user_id.go : env can not be empty COMMON_STREAM")
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_get_user_id.go : start with COMMON_STREAM = [%s]", commonStreamName)

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : internal_get_user_id.go : kinesis client was successfully initialized")
}

func handler(ctx context.Context, request commons.InternalGetUserIdReq) (commons.InternalGetUserIdResp, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "internal_get_user_id.go : start handle request %v", request)

	if request.WarmUpRequest {
		return commons.InternalGetUserIdResp{}, nil
	}

	resp := commons.InternalGetUserIdResp{}

	userId, _, userReportStatus, ok, errStr := commons.Login(request.BuildNum, request.IsItAndroid, request.AccessToken, secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "internal_get_user_id.go : return %s to client", errStr)

		if strings.Contains(errStr, "InvalidAccessTokenClientError") {
			resp.ErrorCode = "InvalidAccessTokenClientError"
			resp.ErrorMessage = "Invalid access token"
			return resp, nil
		}

		if strings.Contains(errStr, "TooOldAppVersionClientError") {
			resp.ErrorCode = "TooOldAppVersionClientError"
			resp.ErrorMessage = "Too old app version"
			return resp, nil
		}

		resp.ErrorCode = "InternalServerError"
		resp.ErrorMessage = "Internal Server Error"
		return resp, nil
	}

	resp.UserId = userId
	if userReportStatus == commons.UserTakePartInReport {
		resp.UserTakePartInReport = true
	}

	anlogger.Debugf(lc, "internal_get_user_id.go : successfully check access token and return userId [%s], userReportStatus [%s] in a response",
		resp.UserId, userReportStatus)

	return resp, nil
}

func main() {
	basicLambda.Start(handler)
}
