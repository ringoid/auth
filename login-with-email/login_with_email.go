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
	"github.com/ringoid/commons"
	"strings"
	"../apimodel"
	"github.com/satori/go.uuid"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var awsDeliveryStreamClient *firehose.Firehose

var deliveryStreamName string
var userProfileTable string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : login_with_email.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : login_with_email.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : login_with_email.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : login_with_email.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "login-with-email-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : login_with_email.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : login_with_email.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : login_with_email.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : aws session was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : login_with_email.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : firehose client was successfully initialized")
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

	anlogger.Debugf(lc, "login_with_email.go : start handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "login_with_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	reqParam, ok, errStr := parseParams(request.Body, lc)
	if !ok {
		anlogger.Errorf(lc, "login_with_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	authSessionId, err := uuid.NewV4()
	if err != nil {
		errStr := commons.InternalServerError
		anlogger.Errorf(lc, "login_with_email.go : error while generate authSessionId : %v", err)
		anlogger.Errorf(lc, "login_with_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	anlogger.Debugf(lc, "login_with_email.go : debug print %v %v %v %v %v", sourceIp, appVersion, isItAndroid, reqParam, authSessionId)

	resp := apimodel.LoginWithEmailResponse{}
	resp.AuthSessionId = authSessionId.String()

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "login_with_email.go : error while marshaling resp object : %v", err)
		return commons.NewServiceResponse(commons.InternalServerError), nil
	}
	anlogger.Debugf(lc, "login_with_email.go : return body=%s", string(body))

	return commons.NewServiceResponse(string(body)), nil
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.LoginWithEmailRequest, bool, string) {
	anlogger.Debugf(lc, "login_with_email.go : parse request body [%s]", params)
	var req apimodel.LoginWithEmailRequest
	err := json.Unmarshal([]byte(params), &req)
	if err != nil {
		anlogger.Errorf(lc, "login_with_email.go : error marshaling required params from the string [%s] : %v", params, err)
		return nil, false, commons.InternalServerError
	}

	if req.Email == "" {
		anlogger.Errorf(lc, "login_with_email.go : empty or nil email request param, req %v", req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	//todo:implement email validation
	anlogger.Debugf(lc, "login_with_email.go : successfully parse request string [%s] to %v", params, req)
	return &req, true, ""
}

//return ok and error string
func updateUserProfile(userId, userProfileTableName string, req *apimodel.UpdateProfileRequest, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "login_with_email.go : start update user profile for userId [%s], profile=%v", userId, req)

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#property":  aws.String(commons.UserProfilePropertyColumnName),
				"#transport": aws.String(commons.UserProfileTransportColumnName),
				"#income":    aws.String(commons.UserProfileIncomeColumnName),
				"#height":    aws.String(commons.UserProfileHeightColumnName),
				"#edu":       aws.String(commons.UserProfileEducationLevelColumnName),
				"#hair":      aws.String(commons.UserProfileHairColorColumnName),
				"#children":  aws.String(commons.UserProfileChildrenColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":propertyV":  {N: aws.String(fmt.Sprintf("%v", req.Property))},
				":transportV": {N: aws.String(fmt.Sprintf("%v", req.Transport))},
				":incomeV":    {N: aws.String(fmt.Sprintf("%v", req.Income))},
				":heightV":    {N: aws.String(fmt.Sprintf("%v", req.Height))},
				":eduV":       {N: aws.String(fmt.Sprintf("%v", req.Education))},
				":hairV":      {N: aws.String(fmt.Sprintf("%v", req.HairColor))},
				":childrenV":  {N: aws.String(fmt.Sprintf("%v", req.Children))},
			},
			Key: map[string]*dynamodb.AttributeValue{
				commons.UserIdColumnName: {
					S: aws.String(userId),
				},
			},
			TableName:        aws.String(userProfileTableName),
			UpdateExpression: aws.String("SET #property = :propertyV, #transport = :transportV, #income = :incomeV, #height = :heightV, #edu = :eduV, #hair = :hairV, #children = :childrenV"),
		}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "login_with_email.go : error update user profile for userId [%s], profile=%v : %v", userId, req, err)
		return false, commons.InternalServerError
	}

	anlogger.Infof(lc, "login_with_email.go : successfully update user profile for userId [%s], settings=%v", userId, req)
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
