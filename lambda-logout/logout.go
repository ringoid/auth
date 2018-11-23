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
	"github.com/satori/go.uuid"
	"time"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/ringoid/commons"
)

var anlogger *commons.Logger
var secretWord string
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string
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
		fmt.Printf("lambda-initialization : logout.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : logout.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : logout.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : logout.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "logout-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : logout.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : logout.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : logout.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : logout.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : logout.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : logout.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : logout.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : logout.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : logout.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : logout.go : firehose client was successfully initialized")

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : logout.go : env can not be empty COMMON_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : logout.go : start with COMMON_STREAM = [%s]", commonStreamName)

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : logout.go : kinesis client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "logout.go : handle request %v", request)

	if commons.IsItWarmUpRequest(request.Body, anlogger, lc) {
		return events.APIGatewayProxyResponse{}, nil
	}

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "logout.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	reqParam, ok := parseParams(request.Body, lc)
	if !ok {
		errStr := commons.WrongRequestParamsClientError
		anlogger.Errorf(lc, "logout.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	userId, ok, errStr := commons.Login(appVersion, isItAndroid, reqParam.AccessToken, secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "logout.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	newSessionToken, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "logout.go : error while generate newSessionToken for userId [%s] : %v", userId, err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: commons.InternalServerError}, nil
	}

	ok, errStr = updateSessionToken(userId, newSessionToken.String(), lc)
	if !ok {
		anlogger.Errorf(lc, "logout.go : userId [%s], return %s to client", userId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	event := commons.NewUserLogoutEvent(userId)
	commons.SendAnalyticEvent(event, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	resp := commons.BaseResponse{}
	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "logout.go : error while marshaling resp object %v for userId [%s] : %v", resp, userId, err)
		anlogger.Errorf(lc, "logout.go : userId [%s], return %s to client", userId, errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: commons.InternalServerError}, nil
	}
	anlogger.Debugf(lc, "logout.go : return body=%s to client, userId [%s]", string(body), userId)
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

//return do we already have such user, ok, errorString if not ok
func updateSessionToken(userId, sessionToken string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "logout.go : update sessionToken [%s] for userId [%s]", sessionToken, userId)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#token":     aws.String(commons.SessionTokenColumnName),
			"#updatedAt": aws.String(commons.TokenUpdatedTimeColumnName),
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
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		TableName:        aws.String(userProfileTable),
		UpdateExpression: aws.String("SET #token = :tV, #updatedAt = :uV"),
	}

	_, err := awsDbClient.UpdateItem(input)

	if err != nil {
		anlogger.Errorf(lc, "logout.go : error update sessionToken [%s] for userId [%s] : %v", sessionToken, userId, err)
		return false, commons.InternalServerError
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
