package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"fmt"
	"os"
	"../sys_log"
	"github.com/aws/aws-lambda-go/events"
	"../apimodel"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/satori/go.uuid"
	"time"
	"strconv"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"crypto/sha1"
)

var anlogger *syslog.Logger
var twilioKey string
var awsDbClient *dynamodb.DynamoDB
var userTableName string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string
var nexmoApiKey string
var nexmoApiSecret string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("start.go : env can not be empty ENV")
		os.Exit(1)
	}
	fmt.Printf("start.go : start with ENV = [%s]", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("start.go : env can not be empty PAPERTRAIL_LOG_ADDRESS")
		os.Exit(1)
	}
	fmt.Printf("start.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "start-auth"))
	if err != nil {
		fmt.Errorf("start.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "start.go : logger was successfully initialized")

	userTableName, ok = os.LookupEnv("USER_TABLE")
	if !ok {
		fmt.Printf("start.go : env can not be empty USER_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "start.go : start with USER_TABLE = [%s]", userTableName)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "start.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "start.go : aws session was successfully initialized")

	twilioKey = apimodel.GetSecret(fmt.Sprintf(apimodel.TwilioSecretKeyBase, env), apimodel.TwilioApiKeyName, awsSession, anlogger, nil)
	nexmoApiKey = apimodel.GetSecret(fmt.Sprintf(apimodel.NexmoSecretKeyBase, env), apimodel.NexmoApiKeyName, awsSession, anlogger, nil)
	nexmoApiSecret = apimodel.GetSecret(fmt.Sprintf(apimodel.NexmoApiSecretKeyBase, env), apimodel.NexmoApiSecretKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "start.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "start.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "start.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "start.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "start.go : handle request %v", request)

	if apimodel.IsItWarmUpRequest(request.Body, anlogger, lc) {
		return events.APIGatewayProxyResponse{}, nil
	}

	reqParam, ok := parseParams(request.Body, lc)
	if !ok {
		strErr := apimodel.WrongRequestParamsClientError
		anlogger.Errorf(lc, "start.go : return %s to client", strErr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: strErr}, nil
	}

	sourceIp := request.RequestContext.Identity.SourceIP
	fullPhone := strconv.Itoa(reqParam.CountryCallingCode) + reqParam.Phone

	userId, ok, errStr := generateUserId(fullPhone, lc)
	if !ok {
		anlogger.Errorf(lc, "start.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	sessionId, err := uuid.NewV4()
	if err != nil {
		strErr := apimodel.InternalServerError
		anlogger.Errorf(lc, "start.go : error while generate sessionId for userId [%s] : %v", userId, err)
		anlogger.Errorf(lc, "start.go : return %s to client", strErr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: strErr}, nil
	}

	customerId, err := uuid.NewV4()
	if err != nil {
		strErr := apimodel.InternalServerError
		anlogger.Errorf(lc, "start.go : error while generate customerId : %v", err)
		anlogger.Errorf(lc, "start.go : return %s to client", strErr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: strErr}, nil
	}

	provider, ok := apimodel.RoutingRuleMap[reqParam.CountryCallingCode]
	if !ok {
		provider = apimodel.Nexmo
	}
	anlogger.Debugf(lc, "start.go : chose verify provide [%s] for userId [%s]", provider, userId)

	userInfo := &apimodel.UserInfo{
		UserId:         userId,
		SessionId:      sessionId.String(),
		Phone:          fullPhone,
		CountryCode:    reqParam.CountryCallingCode,
		PhoneNumber:    reqParam.Phone,
		CustomerId:     customerId.String(),
		VerifyProvider: provider,
	}

	resUserId, resSessionId, resCustomerId, wasCreated, ok, errStr := createUserInfo(userInfo, lc)
	if !ok {
		anlogger.Errorf(lc, "start.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	resp := apimodel.AuthResp{}

	if wasCreated {
		anlogger.Debugf(lc, "start.go : new userId was reserved, userId [%s], sessionId [%s] and customerId [%s]",
			resUserId, resSessionId, resCustomerId)

		resp.SessionId = resSessionId
		resp.CustomerId = resCustomerId
	} else {
		newSessionId, resultUserId, resCustomerId, ok, errStr := updateSessionId(userInfo.Phone, userInfo.SessionId, userInfo.VerifyProvider, lc)
		if !ok {
			anlogger.Errorf(lc, "start.go : return %s to client", errStr)
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
		}
		//we need this coz scope of resultUserId
		resUserId = resultUserId
		resp.SessionId = newSessionId
		resp.CustomerId = resCustomerId
		anlogger.Debugf(lc, "start.go : userId [%s] for such phone [%s] was previously reserved, new sessionId [%s] was generated, customerId [%s]",
			resUserId, userInfo.Phone, resp.SessionId, resp.CustomerId)
	}
	//send analytics event
	event := apimodel.NewUserAcceptTermsEvent(reqParam, sourceIp, resUserId, resCustomerId)
	apimodel.SendAnalyticEvent(event, resUserId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	//send sms
	switch userInfo.VerifyProvider {
	case apimodel.Twilio:
		ok, errorStr := apimodel.StartTwilioVerify(userInfo.CountryCode, userInfo.PhoneNumber, reqParam.Locale, twilioKey, anlogger, lc)
		if !ok {
			anlogger.Errorf(lc, "start.go : return %s to client", errorStr)
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errorStr}, nil
		}
	case apimodel.Nexmo:
		requestId, ok, errorStr := apimodel.StartNexmoVerify(userInfo.CountryCode, userInfo.PhoneNumber, nexmoApiKey, nexmoApiSecret, "Ringoid", userInfo.UserId, anlogger, lc)
		if !ok {
			anlogger.Errorf(lc, "start.go : return %s to client", errorStr)
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errorStr}, nil
		}
		ok, errorStr = updateRequestId(userInfo.Phone, requestId, userInfo.UserId, lc)
		if !ok {
			anlogger.Errorf(lc, "start.go : return %s to client", errorStr)
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errorStr}, nil
		}
	default:
		errorStr := apimodel.InternalServerError
		anlogger.Errorf(lc, "start.go : unsupported verify provider [%s]", userInfo.VerifyProvider)
		anlogger.Errorf(lc, "start.go : return %s to client", errorStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errorStr}, nil
	}

	//send analytics event
	eventStartVerify := apimodel.NewUserVerificationStart(resUserId, userInfo.VerifyProvider, userInfo.CountryCode)
	apimodel.SendAnalyticEvent(eventStartVerify, resUserId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while marshaling resp object : %v", err)
		anlogger.Errorf(lc, "start.go : return %s to client", apimodel.InternalServerError)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	strResp := string(body)
	anlogger.Infof(lc, "start.go : successfully initiate verification for userId [%s], return body=%s to the client", resUserId, strResp)
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: strResp}, nil
}

//return generated userId, was everything ok and error string
func generateUserId(phone string, lc *lambdacontext.LambdaContext) (string, bool, string) {
	anlogger.Debugf(lc, "start.go : generate userId for phone [%s]", phone)
	saltForUserId, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while generate salt for userId, phone [%s] : %v", phone, err)
		return "", false, apimodel.InternalServerError
	}
	sha := sha1.New()
	_, err = sha.Write([]byte(phone))
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while write phone to sha algo, phone [%s] : %v", phone, err)
		return "", false, apimodel.InternalServerError
	}
	_, err = sha.Write([]byte(saltForUserId.String()))
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while write salt to sha algo, phone [%s] : %v", phone, err)
		return "", false, apimodel.InternalServerError
	}
	resultUserId := fmt.Sprintf("%x", sha.Sum(nil))
	anlogger.Debugf(lc, "start.go : successfully generate userId [%s] for phone [%s]", resultUserId, phone)
	return resultUserId, true, ""
}

//return updated sessionId, userId, customerId is everything ok and error string
func updateSessionId(phone, sessionId, provider string, lc *lambdacontext.LambdaContext) (string, string, string, bool, string) {
	anlogger.Debugf(lc, "start.go : update sessionId [%s], provider [%s] for phone [%s]", sessionId, provider, phone)

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#sessionId":     aws.String(apimodel.SessionIdColumnName),
				"#time":          aws.String(apimodel.UpdatedTimeColumnName),
				"#provider":      aws.String(apimodel.VerifyProviderColumnName),
				"#verifyStatus":  aws.String(apimodel.VerificationStatusColumnName),
				"#verifyStartAt": aws.String(apimodel.VerificationStartAtColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":sV": {
					S: aws.String(fmt.Sprint(sessionId)),
				},
				":tV": {
					S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
				},
				":providerV": {
					S: aws.String(provider),
				},
				":verifyStatusV": {
					S: aws.String("start"),
				},
				":verifyStartAtV": {
					N: aws.String(fmt.Sprintf("%v", time.Now().Unix())),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				apimodel.PhoneColumnName: {
					S: aws.String(phone),
				},
			},
			TableName:        aws.String(userTableName),
			UpdateExpression: aws.String("SET #sessionId = :sV, #time = :tV, #provider = :providerV, #verifyStatus = :verifyStatusV, #verifyStartAt = :verifyStartAtV"),
			ReturnValues:     aws.String("ALL_NEW"),
		}

	res, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while update sessionId [%s], provider [%s] for phone [%s] : %v", sessionId, provider, phone, err)
		return "", "", "", false, apimodel.InternalServerError
	}

	resultSessionId := *res.Attributes[apimodel.SessionIdColumnName].S
	resultUserId := *res.Attributes[apimodel.UserIdColumnName].S
	resultCustomerId := *res.Attributes[apimodel.CustomerIdColumnName].S

	anlogger.Debugf(lc, "start.go : successfully update sessionId [%s], provider [%s] for phone [%s], userId [%s] and customerId [%s]", resultSessionId, provider, phone, resultUserId, resultCustomerId)

	return resultSessionId, resultUserId, resultCustomerId, true, ""
}

//return ok and error string
func updateRequestId(phone, requestId, userId string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "start.go : update request id [%s], for phone [%s] for userId [%s]", requestId, phone, userId)
	if len(requestId) == 0 {
		anlogger.Debugf(lc, "start.go : update request id with empty value not allowed, userId [%s], return", userId)
		return true, ""
	}

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#requestId": aws.String(apimodel.VerifyRequestIdColumnName),
				"#time":      aws.String(apimodel.UpdatedTimeColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":rV": {
					S: aws.String(fmt.Sprint(requestId)),
				},
				":tV": {
					S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				apimodel.PhoneColumnName: {
					S: aws.String(phone),
				},
			},
			TableName:        aws.String(userTableName),
			UpdateExpression: aws.String("SET #requestId = :rV, #time = :tV"),
		}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while update request id [%s], for phone [%s] and userId [%s] : %v", requestId, phone, userId, err)
		return false, apimodel.InternalServerError
	}

	anlogger.Debugf(lc, "start.go : successfully update request id [%s] for phone [%s] and userId [%s]", requestId, phone, userId)

	return true, ""
}

//return userId, sessionId,  was user created, was everything ok and error string
func createUserInfo(userInfo *apimodel.UserInfo, lc *lambdacontext.LambdaContext) (userId, sessionId, customerId string, wasCreated, ok bool, errorStr string) {
	anlogger.Debugf(lc, "start.go : reserve userId and customerId, for phone [%s], userInfo=%v", userInfo.Phone, userInfo)

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#userId":    aws.String(apimodel.UserIdColumnName),
				"#sessionId": aws.String(apimodel.SessionIdColumnName),

				"#countryCode":   aws.String(apimodel.CountryCodeColumnName),
				"#phoneNumber":   aws.String(apimodel.PhoneNumberColumnName),
				"#time":          aws.String(apimodel.UpdatedTimeColumnName),
				"#customerId":    aws.String(apimodel.CustomerIdColumnName),
				"#provider":      aws.String(apimodel.VerifyProviderColumnName),
				"#verifyStatus":  aws.String(apimodel.VerificationStatusColumnName),
				"#verifyStartAt": aws.String(apimodel.VerificationStartAtColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":uV": {
					S: aws.String(userInfo.UserId),
				},
				":sV": {
					S: aws.String(fmt.Sprint(userInfo.SessionId)),
				},

				":cV": {
					S: aws.String(strconv.Itoa(userInfo.CountryCode)),
				},
				":pnV": {
					S: aws.String(userInfo.PhoneNumber),
				},
				":tV": {
					S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
				},
				":cIdV": {
					S: aws.String(userInfo.CustomerId),
				},
				":providerV": {
					S: aws.String(userInfo.VerifyProvider),
				},
				":verifyStatusV": {
					S: aws.String("start"),
				},
				":verifyStartAtV": {
					N: aws.String(fmt.Sprintf("%v", time.Now().Unix())),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				apimodel.PhoneColumnName: {
					S: aws.String(userInfo.Phone),
				},
			},
			ConditionExpression: aws.String(fmt.Sprintf("attribute_not_exists(%v)", apimodel.PhoneColumnName)),

			TableName:        aws.String(userTableName),
			UpdateExpression: aws.String("SET #userId = :uV, #sessionId = :sV, #countryCode = :cV, #phoneNumber = :pnV, #time = :tV, #customerId = :cIdV, #provider = :providerV, #verifyStatus = :verifyStatusV, #verifyStartAt = :verifyStartAtV"),
			ReturnValues:     aws.String("ALL_NEW"),
		}

	res, err := awsDbClient.UpdateItem(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				anlogger.Debugf(lc, "start.go : userId was already reserved for this phone number, phone [%s]", userInfo.Phone)
				return "", "", "", false, true, ""
			}
		}
		anlogger.Errorf(lc, "start.go : error while reserve userId and customerId for phone [%s], userInfo=%v : %v", userInfo.Phone, userInfo, err)
		return "", "", "", false, false, apimodel.InternalServerError
	}

	resUserId := *res.Attributes[apimodel.UserIdColumnName].S
	resSessionId := *res.Attributes[apimodel.SessionIdColumnName].S
	resCustomerId := *res.Attributes[apimodel.CustomerIdColumnName].S
	anlogger.Debugf(lc, "start.go : successfully reserve userId [%s] and customerId [%s] for phone [%s]", resUserId, resCustomerId, userInfo.Phone)
	return resUserId, resSessionId, resCustomerId, true, true, ""
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.StartReq, bool) {
	var req apimodel.StartReq
	err := json.Unmarshal([]byte(params), &req)

	if err != nil {
		anlogger.Errorf(lc, "start.go : error parsing required params from the body string [%s] : %v", params, err)
		return nil, false
	}

	if req.CountryCallingCode == 0 || req.Phone == "" || req.DateTimeTermsAndConditions == 0 ||
		req.DateTimePrivacyNotes == 0 || req.DateTimeLegalAge == 0 {
		anlogger.Errorf(lc, "start.go : one of the required param is nil, req %v", req)
		return nil, false
	}

	anlogger.Debugf(lc, "start.go : successfully parse param string [%s] to request=%v", params, req)
	return &req, true
}

func main() {
	basicLambda.Start(handler)
}
