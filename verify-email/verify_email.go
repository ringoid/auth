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
	"strconv"
	"github.com/satori/go.uuid"
	"github.com/dgrijalva/jwt-go"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var awsDeliveryStreamClient *firehose.Firehose

var secretWord string

var deliveryStreamName string
var userProfileTable string

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
		fmt.Printf("lambda-initialization : verify_email.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : verify_email.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : verify_email.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : verify_email.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "verify-email-auth"), apimodel.IsDebugLogEnabled)
	if err != nil {
		fmt.Errorf("lambda-initialization : verify_email.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : verify_email.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : verify_email.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : verify_email.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	emailAuthTable, ok = os.LookupEnv("EMAIL_AUTH_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : login_with_email.go : env can not be empty EMAIL_AUTH_TABLE")
	}
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : start with EMAIL_AUTH_TABLE = [%s]", emailAuthTable)

	authConfirmTable, ok = os.LookupEnv("AUTH_CONFIRM_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : login_with_email.go : env can not be empty AUTH_CONFIRM_TABLE")
	}
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : start with AUTH_CONFIRM_TABLE = [%s]", authConfirmTable)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : verify_email.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : verify_email.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : verify_email.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : verify_email.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : verify_email.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : verify_email.go : firehose client was successfully initialized")
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
	//sourceIp := request.Headers["x-forwarded-for"]

	anlogger.Debugf(lc, "verify_email.go : start handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "verify_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = commons.CheckAppVersion(appVersion, isItAndroid, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "verify_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	reqParam, ok, errStr := parseParams(request.Body, lc)
	if !ok {
		anlogger.Errorf(lc, "verify_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = baseCheck(reqParam.Email, reqParam.AuthSessionId, lc)
	if !ok {
		anlogger.Errorf(lc, "verify_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	piCode, _ := strconv.Atoi(reqParam.PinCode)
	userId, ok, errStr := completeEmailConfirmation(reqParam.Email, reqParam.AuthSessionId, piCode, lc)
	if !ok {
		anlogger.Errorf(lc, "verify_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	newSessionToken, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "verify_email.go : error while generate new sessionToken for userId [%s] : %v", userId, err)
		errStr = commons.InternalServerError
		anlogger.Errorf(lc, "verify_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = apimodel.SwithCurrentAccessToken(userId, newSessionToken.String(), userProfileTable, awsDbClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "verify_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	//create access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		commons.AccessTokenUserIdClaim:       userId,
		commons.AccessTokenSessionTokenClaim: newSessionToken.String(),
	})

	tokenToString, err := accessToken.SignedString([]byte(secretWord))
	if err != nil {
		errStr = commons.InternalServerError
		anlogger.Errorf(lc, "verify_email.go : error sign the token for userId [%s], return %s to the client : %v", userId, errStr, err)
		return commons.NewServiceResponse(errStr), nil
	}

	resp := apimodel.VerifyEmailResponse{}
	resp.AccessToken = tokenToString

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "verify_email.go : error while marshaling resp object : %v", err)
		return commons.NewServiceResponse(commons.InternalServerError), nil
	}
	anlogger.Debugf(lc, "verify_email.go : return body=%s", string(body))

	return commons.NewServiceResponse(string(body)), nil
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.VerifyEmailRequest, bool, string) {
	anlogger.Debugf(lc, "verify_email.go : parse request body [%s]", params)
	var req apimodel.VerifyEmailRequest
	err := json.Unmarshal([]byte(params), &req)
	if err != nil {
		anlogger.Errorf(lc, "verify_email.go : error marshaling required params from the string [%s] : %v", params, err)
		return nil, false, commons.InternalServerError
	}

	if req.Email == "" {
		anlogger.Errorf(lc, "verify_email.go : empty or nil email request param, req %v", req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	if req.AuthSessionId == "" {
		anlogger.Errorf(lc, "verify_email.go : empty or nil authSessionId request param, req %v", req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	if req.PinCode == "" {
		anlogger.Errorf(lc, "verify_email.go : empty or nil pinCode request param, req %v", req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	_, err = strconv.Atoi(req.PinCode)
	if err != nil {
		anlogger.Errorf(lc, "verify_email.go : pin code is not int number, pin [%v]", req.PinCode)
		return nil, false, commons.WrongRequestParamsClientError
	}

	//todo:implement email validation
	anlogger.Debugf(lc, "verify_email.go : successfully parse request string [%s] to %v", params, req)
	return &req, true, ""
}

//return userId, ok and error string
func completeEmailConfirmation(email, authSessionId string, pin int, lc *lambdacontext.LambdaContext) (string, bool, string) {
	anlogger.Debugf(lc, "verify_email.go : complete email confirmation, email [%s], pin [%d], auth session id [%s]",
		email, pin, authSessionId)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#pin":                     aws.String(commons.AuthConfirmPinColumnName),
			"#authSessionId":           aws.String(commons.AuthConfirmSessionIdColumnName),
			"#emailConfirmationStatus": aws.String(commons.AuthConfirmStatusColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":authStatusStartedV": {
				S: aws.String(commons.AuthConfirmStatusStartedValue),
			},
			":pinV": {
				N: aws.String(fmt.Sprintf("%d", pin)),
			},
			":authSessionIdV": {
				S: aws.String(authSessionId),
			},
			":emailConfirmationStatusV": {
				S: aws.String(commons.AuthConfirmStatusCompleteValue),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			commons.AuthConfirmMailColumnName: {
				S: aws.String(email),
			},
		},
		ConditionExpression: aws.String("#emailConfirmationStatus = :authStatusStartedV AND #authSessionId = :authSessionIdV AND #pin = :pinV"),
		TableName:           aws.String(authConfirmTable),
		UpdateExpression:    aws.String("SET #emailConfirmationStatus = :emailConfirmationStatusV"),
		ReturnValues:        aws.String("ALL_NEW"),
	}

	res, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "verify_email.go : error to complete confirmation email [%s], pin [%d] and auth session id [%s] : %v",
			email, pin, authSessionId, err)
		return "", false, commons.WrongPinCodeClientError
	}

	propertyP, ok := res.Attributes[commons.AuthConfirmUserIdColumnName]
	if !ok || propertyP.S == nil {
		anlogger.Errorf(lc, "verify_email.go : error to complete confirmation, userId is empty, email [%s], pin [%d] and auth session id [%s] : %v",
			email, pin, authSessionId, err)
		return "", false, commons.EmailInvalidVerificationClientError
	}
	userId := *propertyP.S

	anlogger.Infof(lc, "verify_email.go : successfully complete confirmation with email [%s], pin [%d] and auth session id [%s] with userId [%s]",
		email, pin, authSessionId, userId)

	return userId, true, ""
}

//return ok and error string
func baseCheck(email, authSessionId string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "verify_email.go : base check that we can proceed with pin, for email [%s] and "+
		"authSessionId [%s]", email, authSessionId)

	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			commons.AuthConfirmMailColumnName: {
				S: aws.String(email),
			},
		},
		TableName:      aws.String(authConfirmTable),
		ConsistentRead: aws.Bool(true),
	}

	result, err := awsDbClient.GetItem(input)
	if err != nil {
		anlogger.Errorf(lc, "verify_email.go : error get email confirm state for email [%s] : %v", email, err)
		return false, commons.InternalServerError
	}

	if len(result.Item) == 0 {
		anlogger.Errorf(lc, "get_profile.go : there is no email confirm record with email [%s]", email)
		return false, commons.EmailInvalidVerificationClientError
	}

	ok, authSId := getStringValueProfileProperty(commons.AuthConfirmSessionIdColumnName, result, lc)
	if !ok || authSessionId != authSId {
		anlogger.Errorf(lc, "get_profile.go : there is no authSessionId in email confirm record or they are different, email [%s], "+
			"session id stored in DB [%s], target session id [%s]", email, authSId, authSessionId)
		return false, commons.EmailInvalidVerificationClientError
	}

	ok, confirmState := getStringValueProfileProperty(commons.AuthConfirmStatusColumnName, result, lc)
	if !ok || confirmState != commons.AuthConfirmStatusStartedValue {
		anlogger.Errorf(lc, "get_profile.go : there is no confirmation status in email confirm record or they are different, email [%s], "+
			"state stored in DB [%s], target state id [%s]", email, confirmState, commons.AuthConfirmStatusStartedValue)
		return false, commons.EmailInvalidVerificationClientError
	}

	return true, ""
}

//ok and string value
func getStringValueProfileProperty(propertyName string, result *dynamodb.GetItemOutput, lc *lambdacontext.LambdaContext) (bool, string) {
	profilePropertyP, ok := result.Item[propertyName]
	if ok {
		if profilePropertyP.S != nil {
			return true, *profilePropertyP.S
		}
	}
	return false, ""
}

func main() {
	basicLambda.Start(handler)
}
