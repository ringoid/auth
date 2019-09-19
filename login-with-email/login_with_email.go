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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"math/rand"
	"github.com/mailgun/mailgun-go"
	"time"
)

const (
	ringoidAppDomain = "ringoid.app"
	emailSender      = "Ringoid Support <support@ringoid.com>"
	emailTemplate    = "verification_code"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var awsDeliveryStreamClient *firehose.Firehose

var deliveryStreamName string
var emailAuthTable string
var authConfirmTable string
var mailgunApiKey string

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

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "login-with-email-auth"), apimodel.IsDebugLogEnabled)
	if err != nil {
		fmt.Errorf("lambda-initialization : login_with_email.go : error during startup : %v\n", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : logger was successfully initialized")

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
		anlogger.Fatalf(nil, "lambda-initialization : login_with_email.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : login_with_email.go : aws session was successfully initialized")

	mailgunApiKey = commons.GetSecret(fmt.Sprintf(commons.MailGunApiKeyBase, env), commons.MailGunApiKeyName, awsSession, anlogger, nil)

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
	//sourceIp := request.Headers["x-forwarded-for"]

	anlogger.Debugf(lc, "login_with_email.go : start handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)

	ok, errStr = commons.CheckAppVersion(appVersion, isItAndroid, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "login_with_email.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

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

	resp := apimodel.LoginWithEmailResponse{}
	resp.AuthSessionId = authSessionId.String()

	ok, errStr = tryUpdateAuthStatus(reqParam.Email, authSessionId.String(), lc)
	if !ok {
		if len(errStr) != 0 {
			anlogger.Errorf(lc, "login_with_email.go : return %s to client", errStr)
			return commons.NewServiceResponse(errStr), nil
		}

		userId, ok, errStr := readUserIdFromEmailAuth(reqParam.Email, lc)
		if !ok {
			anlogger.Errorf(lc, "login_with_email.go : return %s to client", errStr)
			return commons.NewServiceResponse(errStr), nil
		}

		pinCode := rand.Intn(89999) + 10000
		ok, errStr = startEmailConfirmation(userId, reqParam.Email, authSessionId.String(), pinCode, lc)
		if !ok {
			anlogger.Errorf(lc, "login_with_email.go : return %s to client", errStr)
			return commons.NewServiceResponse(errStr), nil
		}

		ok, errStr = sendEmailWithPin(reqParam.Email, reqParam.Locale, pinCode, lc)
		if !ok {
			anlogger.Errorf(lc, "login_with_email.go : return %s to client", errStr)
			return commons.NewServiceResponse(errStr), nil
		}

		resp.ErrorCode = commons.ErrorCodeEmailNotVerifiedClientError
		resp.ErrorMessage = commons.ErrorMessageEmailNotVerifiedClientError
	}

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

//return userId, ok and error string
func readUserIdFromEmailAuth(email string, lc *lambdacontext.LambdaContext) (string, bool, string) {
	anlogger.Debugf(lc, "login_with_email.go : read userId for email [%s]", email)

	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			commons.EmailAuthMailColumnName: {
				S: aws.String(email),
			},
		},
		TableName:      aws.String(emailAuthTable),
		ConsistentRead: aws.Bool(true),
	}

	result, err := awsDbClient.GetItem(input)
	if err != nil {
		anlogger.Errorf(lc, "login_with_email.go : error get email auth record for email [%s] : %v", email, err)
		return "", false, commons.InternalServerError
	}

	if len(result.Item) == 0 {
		anlogger.Errorf(lc, "login_with_email.go : there is no email auth record with email [%s]", email)
		return "", false, commons.InternalServerError
	}

	ok, userId := getStringValueProfileProperty(commons.EmailAuthUserIdColumnName, result, lc)
	if !ok {
		anlogger.Errorf(lc, "login_with_email.go : there is no userId in email auth record, email [%s]", email)
		return "", false, commons.InternalServerError
	}

	anlogger.Debugf(lc, "login_with_email.go : successfully read userId [%s] for email [%s]", userId, email)
	return userId, true, ""
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

//return ok and error string
func tryUpdateAuthStatus(email, authSessionId string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "login_with_email.go : update auth status to started state, email [%s], auth session id [%s]",
		email, authSessionId)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#authStatus":    aws.String(commons.EmailAuthStatusColumnName),
			"#authSessionId": aws.String(commons.EmailAuthSessionIdColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":authStatusV": {
				S: aws.String(commons.EmailAuthStatusStartedValue),
			},
			":authSessionIdV": {
				S: aws.String(authSessionId),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			commons.EmailAuthMailColumnName: {
				S: aws.String(email),
			},
		},
		ConditionExpression: aws.String(
			fmt.Sprintf("attribute_not_exists(%s) OR #authStatus = :authStatusV",
				commons.EmailAuthSessionIdColumnName)),
		TableName:        aws.String(emailAuthTable),
		UpdateExpression: aws.String("SET #authStatus = :authStatusV, #authSessionId = :authSessionIdV"),
	}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				anlogger.Warnf(lc, "login_with_email.go : warning, try to login with email which already exists, email [%s]", email)
				return false, ""
			default:
				anlogger.Errorf(lc, "login_with_email.go : error to login with email [%s] : %v", email, aerr)
				return false, commons.InternalServerError
			}
		}
		anlogger.Errorf(lc, "login_with_email.go : error to login with email [%s] : %v", email, err)
		return false, commons.InternalServerError
	}

	anlogger.Infof(lc, "login_with_email.go : successfully update auth status to started state, email [%s], auth session id [%s]",
		email, authSessionId)
	return true, ""
}

func startEmailConfirmation(userId, email, authSessionId string, pin int, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "login_with_email.go : start email confirmation, email [%s], userId [%s], pin [%d], auth session id [%s]",
		email, userId, pin, authSessionId)

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#pin":                     aws.String(commons.AuthConfirmPinColumnName),
			"#authSessionId":           aws.String(commons.AuthConfirmSessionIdColumnName),
			"#emailConfirmationStatus": aws.String(commons.AuthConfirmStatusColumnName),
			"#userId":                  aws.String(commons.AuthConfirmUserIdColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":pinV": {
				N: aws.String(fmt.Sprintf("%d", pin)),
			},
			":authSessionIdV": {
				S: aws.String(authSessionId),
			},
			":emailConfirmationStatusV": {
				S: aws.String(commons.AuthConfirmStatusStartedValue),
			},
			":userIdV": {
				S: aws.String(userId),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			commons.AuthConfirmMailColumnName: {
				S: aws.String(email),
			},
		},
		TableName:        aws.String(authConfirmTable),
		UpdateExpression: aws.String("SET #pin = :pinV, #authSessionId = :authSessionIdV, #emailConfirmationStatus = :emailConfirmationStatusV, #userId = :userIdV"),
	}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "login_with_email.go : error to start confirmation email [%s], userId [%s], pin [%d] and auth session id [%s] : %v",
			email, userId, pin, authSessionId, err)
		return false, commons.InternalServerError
	}

	anlogger.Infof(lc, "login_with_email.go : successfully start confirmation with email [%s], userId [%s], pin [%d] and auth session id [%s]",
		email, userId, pin, authSessionId)

	return true, ""
}

func sendEmailWithPin(email, locale string, pin int, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Infof(lc, "login_with_email.go : send verification code [%d] for [%s]", pin, email)
	mg := mailgun.NewMailgun(ringoidAppDomain, mailgunApiKey)
	mg.SetAPIBase(mailgun.APIBaseEU)
	subject := fmt.Sprintf("%d is your verification code", pin)
	if locale == "ru" {
		subject = fmt.Sprintf("%d Ваш код верификации", pin)
	}

	message := mg.NewMessage(emailSender, subject, "", email)
	message.SetTemplate(emailTemplate)
	message.AddVariable("code", pin)
	if locale == "ru" {
		message.AddVariable("ru", true)
	} else {
		message.AddVariable("en", true)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// Send the message	with a 10 second timeout
	resp, id, err := mg.Send(ctx, message)

	if err != nil {
		anlogger.Errorf(lc, "login_with_email.go : error sending verification code [%d] for [%s] : %v", pin, email, err)
		return false, commons.InternalServerError
	}

	anlogger.Infof(lc, "login_with_email.go : successfully sent verification code [%d] for [%s] with id [%s] and resp [%s]",
		pin, email, id, resp)

	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
