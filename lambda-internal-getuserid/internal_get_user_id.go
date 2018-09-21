package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"../sys_log"
	"../apimodel"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

var anlogger *syslog.Logger
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var secretWord string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("internal_get_user_id.go : env can not be empty ENV")
		os.Exit(1)
	}
	fmt.Printf("internal_get_user_id.go : start with ENV = [%s]", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("internal_get_user_id.go : env can not be empty PAPERTRAIL_LOG_ADDRESS")
		os.Exit(1)
	}
	fmt.Printf("internal_get_user_id.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "internal-get-user-id-auth"))
	if err != nil {
		fmt.Errorf("internal_get_user_id.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "internal_get_user_id.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("internal_get_user_id.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "internal_get_user_id.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "internal_get_user_id.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "internal_get_user_id.go : aws session was successfully initialized")

	secretWord = apimodel.GetSecret(fmt.Sprintf(apimodel.SecretWordKeyBase, env), apimodel.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "internal_get_user_id.go : dynamodb client was successfully initialized")
}

func handler(ctx context.Context, request apimodel.InternalGetUserIdReq) (apimodel.InternalGetUserIdResp, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "internal_get_user_id.go : start handle request %v", request)

	if request.WarmUpRequest {
		return apimodel.InternalGetUserIdResp{}, nil
	}

	resp := apimodel.InternalGetUserIdResp{}

	userId, sessionToken, ok, errStr := apimodel.DecodeToken(request.AccessToken, secretWord, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "internal_get_user_id.go : return %s to client", errStr)
		resp.ErrorCode = "InternalServerError"
		resp.ErrorMessage = "Internal Server Error"
		return resp, nil
	}

	valid, ok, errStr := apimodel.IsSessionValid(userId, sessionToken, userProfileTable, awsDbClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "internal_get_user_id.go : return %s to client", errStr)
		resp.ErrorCode = "InternalServerError"
		resp.ErrorMessage = "Internal Server Error"
		return resp, nil
	}

	if !valid {
		anlogger.Errorf(lc, "internal_get_user_id.go : return %s to client", apimodel.InvalidAccessTokenClientError)
		resp.ErrorCode = "InvalidAccessTokenClientError"
		resp.ErrorMessage = "Invalid Access Token"
		return resp, nil
	}

	resp.UserId = userId

	anlogger.Debugf(lc, "internal_get_user_id.go : successfully check access token and return userId response %v", resp)

	return resp, nil
}

func main() {
	basicLambda.Start(handler)
}
