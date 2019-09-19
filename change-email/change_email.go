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
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var awsDeliveryStreamClient *firehose.Firehose
var awsKinesisClient *kinesis.Kinesis

var deliveryStreamName string
var userProfileTable string
var secretWord string
var commonStreamName string
var emailAuthTable string
var authConfirmTable string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : change_email.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : change_email.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : change_email.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : change_email.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "change-email-auth"), apimodel.IsDebugLogEnabled)
	if err != nil {
		fmt.Errorf("lambda-initialization : change_email.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : change_email.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : change_email.go : env can not be empty COMMON_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : start with DELIVERY_STREAM = [%s]", commonStreamName)

	emailAuthTable, ok = os.LookupEnv("EMAIL_AUTH_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : change_email.go : env can not be empty EMAIL_AUTH_TABLE")
	}
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : start with EMAIL_AUTH_TABLE = [%s]", emailAuthTable)

	authConfirmTable, ok = os.LookupEnv("AUTH_CONFIRM_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : change_email.go : env can not be empty AUTH_CONFIRM_TABLE")
	}
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : start with AUTH_CONFIRM_TABLE = [%s]", authConfirmTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : change_email.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : dynamodb client was successfully initialized")

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : kinesis client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : change_email.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : change_email.go : firehose client was successfully initialized")
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

	anlogger.Debugf(lc, "change_email.go : start handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "change_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	reqParam, ok, errStr := parseParams(request.Body, lc)
	if !ok {
		anlogger.Errorf(lc, "change_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	userId, _, _, ok, errStr := commons.Login(appVersion, isItAndroid, reqParam.AccessToken, secretWord, userProfileTable, commonStreamName, awsDbClient, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "update_settings.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	currentEmail, ok, errStr := currentEmail(userId, lc)
	if !ok {
		anlogger.Errorf(lc, "change_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = tryUpdateAuthStatusForNewEmail(userId, reqParam.NewEmail, lc)
	if !ok {
		anlogger.Errorf(lc, "change_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = cleanEmailState(userId, currentEmail, reqParam.NewEmail, lc)
	if !ok {
		anlogger.Errorf(lc, "change_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	changeEmailEvent := commons.NewUserChangeEmailEvent(userId, currentEmail, reqParam.NewEmail, sourceIp)
	commons.SendAnalyticEvent(changeEmailEvent, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	resp := commons.BaseResponse{}

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "change_email.go : error while marshaling resp object : %v", err)
		return commons.NewServiceResponse(commons.InternalServerError), nil
	}
	anlogger.Debugf(lc, "change_email.go : return body=%s", string(body))

	anlogger.Infof(lc, "change_email.go : successfully change old email [%s] to new one [%s] for userId [%s]",
		currentEmail, reqParam.NewEmail, userId)

	return commons.NewServiceResponse(string(body)), nil
}

//return current email, ok and error string
func currentEmail(userId string, lc *lambdacontext.LambdaContext) (string, bool, string) {
	anlogger.Debugf(lc, "change_email.go : fetch current email for userId [%s]", userId)

	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		TableName:      aws.String(userProfileTable),
		ConsistentRead: aws.Bool(true),
	}

	result, err := awsDbClient.GetItem(input)
	if err != nil {
		anlogger.Errorf(lc, "change_email.go : error fetch current email for userId [%s] : %v", userId, err)
		return "", false, commons.InternalServerError
	}

	if len(result.Item) == 0 {
		anlogger.Errorf(lc, "change_email.go : there is no such user in DB, userId [%s]", userId)
		return "", false, commons.InternalServerError
	}

	var email string
	attrValue := result.Item[commons.UserEmailColumnName]
	if attrValue != nil {
		email = *attrValue.S
		anlogger.Debugf(lc, "change_email.go : found current email [%s] for userId [%s]", email, userId)
	} else {
		anlogger.Debugf(lc, "change_email.go : there is no current email for userId [%s]", userId)
	}

	return email, true, ""
}

//return ok and error string
func tryUpdateAuthStatusForNewEmail(userId, email string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "change_email.go : update auth status to account created state, for userId [%s] and email [%s]",
		userId, email)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#authStatus":    aws.String(commons.EmailAuthStatusColumnName),
			"#authSessionId": aws.String(commons.EmailAuthSessionIdColumnName),
			"#userId":        aws.String(commons.EmailAuthUserIdColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":authStatusStartedV": {
				S: aws.String(commons.EmailAuthStatusStartedValue),
			},
			":authStatusV": {
				S: aws.String(commons.EmailAuthStatusAccountCreatedValue),
			},
			":authSessionIdV": {
				S: aws.String("CHANGE_EMAIL"),
			},
			":userIdV": {
				S: aws.String(userId),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			commons.EmailAuthMailColumnName: {
				S: aws.String(email),
			},
		},
		ConditionExpression: aws.String(
			fmt.Sprintf("attribute_not_exists(%s) OR #authStatus = :authStatusStartedV",
				commons.EmailAuthSessionIdColumnName)),
		TableName:        aws.String(emailAuthTable),
		UpdateExpression: aws.String("SET #authStatus = :authStatusV, #authSessionId = :authSessionIdV, #userId = :userIdV"),
	}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				anlogger.Errorf(lc, "change_email.go : error, try to change for already exist email [%s] for userId [%s]", email, userId)
				return false, commons.EmailAlreadyInUseClientError
			default:
				anlogger.Errorf(lc, "change_email.go : error to change for already exist email [%s] for userId [%s] : %v", email, userId, aerr)
				return false, commons.InternalServerError
			}
		}
		anlogger.Errorf(lc, "change_email.go : error to change for already exist email [%s] for userId [%s] : %v", email, userId, err)
		return false, commons.InternalServerError
	}

	anlogger.Debugf(lc, "change_email.go : successfully change email [%s] for userId [%s] in EmailAuth table", email, userId)
	return true, ""
}

func cleanEmailState(userId, oldEmail, newEmail string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "change_email.go : clean email state from old [%s] for new one [%s] for userId [%s]", oldEmail, newEmail, userId)

	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			emailAuthTable: {
				{
					DeleteRequest: &dynamodb.DeleteRequest{
						Key: map[string]*dynamodb.AttributeValue{
							commons.EmailAuthMailColumnName: {
								S: aws.String(oldEmail),
							},
						},
					},
				},
			},
			authConfirmTable: {
				{
					DeleteRequest: &dynamodb.DeleteRequest{
						Key: map[string]*dynamodb.AttributeValue{
							commons.AuthConfirmMailColumnName: {
								S: aws.String(oldEmail),
							},
						},
					},
				},
			},
		},
	}

	_, err := awsDbClient.BatchWriteItem(input)
	if err != nil {
		anlogger.Errorf(lc, "change_email.go : error clean email state for old email [%s] for userId [%s] : %v", oldEmail, userId, err)
		return false, commons.InternalServerError
	}

	inputU := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#email": aws.String(commons.UserEmailColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":emailV": {
				S: aws.String(newEmail),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		TableName:        aws.String(userProfileTable),
		UpdateExpression: aws.String("SET #email = :emailV"),
	}

	_, err = awsDbClient.UpdateItem(inputU)
	if err != nil {
		anlogger.Errorf(lc, "change_email.go : error update email [%s] for userId [%s] : %v", newEmail, userId, err)
		return false, commons.InternalServerError
	}

	anlogger.Debugf(lc, "change_email.go : successfully clean email state from old [%s] for new one [%s] for userId [%s]", oldEmail, newEmail, userId)
	return true, ""
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.ChangeEmailRequest, bool, string) {
	anlogger.Debugf(lc, "change_email.go : parse request body [%s]", params)
	var req apimodel.ChangeEmailRequest
	err := json.Unmarshal([]byte(params), &req)
	if err != nil {
		anlogger.Errorf(lc, "change_email.go : error marshaling required params from the string [%s] : %v", params, err)
		return nil, false, commons.InternalServerError
	}

	if req.NewEmail == "" {
		anlogger.Errorf(lc, "change_email.go : empty or nil newEmail request param, req %v", req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	//todo:implement email validation
	anlogger.Debugf(lc, "change_email.go : successfully parse request string [%s] to %v", params, req)
	return &req, true, ""
}

func main() {
	basicLambda.Start(handler)
}
