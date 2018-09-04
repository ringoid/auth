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
	"github.com/satori/go.uuid"
	"time"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

var anlogger *syslog.Logger
var secretWord string
var awsDbClient *dynamodb.DynamoDB
var userTableName string
var userProfileTable string
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
		fmt.Printf("logout.go : env can not be empty ENV")
		os.Exit(1)
	}
	fmt.Printf("logout.go : start with ENV = [%s]", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("logout.go : env can not be empty PAPERTRAIL_LOG_ADDRESS")
		os.Exit(1)
	}
	fmt.Printf("logout.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "complete-auth"))
	if err != nil {
		fmt.Errorf("logout.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "logout.go : logger was successfully initialized")

	userTableName, ok = os.LookupEnv("USER_TABLE")
	if !ok {
		fmt.Printf("logout.go : env can not be empty USER_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "logout.go : start with USER_TABLE = [%s]", userTableName)

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		fmt.Printf("logout.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "logout.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "logout.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "logout.go : aws session was successfully initialized")

	secretWord = apimodel.GetSecret(fmt.Sprintf(apimodel.SecretWordKeyBase, env), apimodel.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "logout.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "logout.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "logout.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "logout.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "logout.go : handle request %v", request)

	reqParam, ok := parseParams(request.Body, lc)
	if !ok {
		errStr := apimodel.WrongRequestParamsClientError
		anlogger.Errorf(lc, "logout.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	userId, sessionToken, ok, errStr := apimodel.DecodeToken(reqParam.AccessToken, secretWord, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "logout.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	valid, ok, errStr := apimodel.IsSessionValid(userId, sessionToken, userProfileTable, awsDbClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "logout.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	if !valid {
		errStr = apimodel.InvalidAccessTokenClientError
		anlogger.Errorf(lc, "logout.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	newSessionToken, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "logout.go : error while generate newSessionToken for userId [%s] : %v", userId, err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}

	ok, errStr = updateSessionToken(userId, newSessionToken.String(), lc)
	if !ok {
		anlogger.Errorf(lc, "logout.go : userId [%s], return %s to client", userId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	event := apimodel.NewUserLogoutEvent(userId)
	apimodel.SendAnalyticEvent(event, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	resp := apimodel.BaseResponse{}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "logout.go : error while marshaling resp object %v for userId [%s] : %v", resp, userId, err)
		anlogger.Errorf(lc, "logout.go : userId [%s], return %s to client", userId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	anlogger.Debugf(lc, "logout.go : return body=%s to client, userId [%s]", string(body), userId)
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

//return do we already have such user, ok, errorString if not ok
func updateSessionToken(userId, sessionToken string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "logout.go : update sessionToken [%s] for userId [%s]", sessionToken, userId)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#token":     aws.String(apimodel.SessionTokenColumnName),
			"#updatedAt": aws.String(apimodel.TokenUpdatedTimeColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":tV": {
				S: aws.String(sessionToken),
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
		TableName:        aws.String(userProfileTable),
		UpdateExpression: aws.String("SET #token = :tV, #updatedAt = :uV"),
	}

	_, err := awsDbClient.UpdateItem(input)

	if err != nil {
		anlogger.Errorf(lc, "logout.go : error update sessionToken [%s] for userId [%s] : %v", sessionToken, userId, err)
		return false, apimodel.InternalServerError
	}

	anlogger.Debugf(lc, "logout.go : successfully update sessionToken [%s] for userId [%s]", sessionToken, userId)
	return true, ""
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.LogoutReq, bool) {
	var req apimodel.LogoutReq
	err := json.Unmarshal([]byte(params), &req)

	if err != nil {
		anlogger.Errorf(lc, "logout.go : error unmarshal required params from the string %s : %v", params, err)
		return nil, false
	}

	if req.AccessToken == "" {
		anlogger.Errorf(lc, "logout.go : one of the required param is nil or empty, req %v", req)
		return nil, false
	}

	return &req, true
}

func main() {
	basicLambda.Start(handler)
}
