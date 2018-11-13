package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"fmt"
	"os"
	"../sys_log"
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
	"github.com/aws/aws-sdk-go/service/sqs"
)

var anlogger *syslog.Logger
var awsDbClient *dynamodb.DynamoDB
var userTableName string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string
var asyncTaskQueue string
var awsSqsClient *sqs.SQS

const defaultMaxTimeToCompleteVerificationInSec = 300

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : internal_start.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : internal_start.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : internal_start.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : internal_start.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "internal-start-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : internal_start.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_start.go : logger was successfully initialized")

	userTableName, ok = os.LookupEnv("USER_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : internal_start.go : env can not be empty USER_TABLE")
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_start.go : start with USER_TABLE = [%s]", userTableName)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : internal_start.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_start.go : aws session was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : internal_start.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : internal_start.go : env can not be empty DELIVERY_STREAM")
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_start.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : internal_start.go : firehose client was successfully initialized")

	asyncTaskQueue, ok = os.LookupEnv("ASYNC_TASK_SQS_QUEUE")
	if !ok {
		fmt.Printf("lambda-initialization : internal_start.go : env can not be empty ASYNC_TASK_SQS_QUEUE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_start.go : start with ASYNC_TASK_SQS_QUEUE = [%s]", asyncTaskQueue)

	awsSqsClient = sqs.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : internal_start.go : sqs client was successfully initialized")
}

func handler(ctx context.Context, request apimodel.StartReq) (apimodel.AuthResp, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "internal_start.go : handle request %v", request)

	sourceIp := "fake_source_api"
	fullPhone := strconv.Itoa(request.CountryCallingCode) + request.Phone

	userId, ok, errStr := generateUserId(fullPhone, lc)
	if !ok {
		anlogger.Errorf(lc, "internal_start.go : return %s to client", errStr)
		return apimodel.AuthResp{}, nil
	}

	sessionId, err := uuid.NewV4()
	if err != nil {
		strErr := apimodel.InternalServerError
		anlogger.Errorf(lc, "internal_start.go : error while generate sessionId for userId [%s] : %v", userId, err)
		anlogger.Errorf(lc, "internal_start.go : return %s to client", strErr)
		return apimodel.AuthResp{}, nil
	}

	customerId, err := uuid.NewV4()
	if err != nil {
		strErr := apimodel.InternalServerError
		anlogger.Errorf(lc, "internal_start.go : error while generate customerId : %v", err)
		anlogger.Errorf(lc, "internal_start.go : return %s to client", strErr)
		return apimodel.AuthResp{}, nil
	}

	provider, ok := apimodel.RoutingRuleMap[request.CountryCallingCode]
	if !ok {
		//used default provider
		provider = apimodel.RoutingRuleMap[-1]
	}
	anlogger.Debugf(lc, "internal_start.go : chose verify provide [%s] for userId [%s]", provider, userId)

	userInfo := &apimodel.UserInfo{
		UserId:         userId,
		SessionId:      sessionId.String(),
		Phone:          fullPhone,
		CountryCode:    request.CountryCallingCode,
		PhoneNumber:    request.Phone,
		CustomerId:     customerId.String(),
		VerifyProvider: provider,
		Locale:         request.Locale,
		DeviceModel:    request.DeviceModel,
		OsVersion:      request.OsVersion,
	}

	resUserId, resSessionId, resCustomerId, wasCreated, ok, errStr := createUserInfo(userInfo, true, lc)
	if !ok {
		anlogger.Errorf(lc, "internal_start.go : return %s to client", errStr)
		return apimodel.AuthResp{}, nil
	}

	resp := apimodel.AuthResp{}

	if wasCreated {
		anlogger.Infof(lc, "internal_start.go : new userId was reserved, userId [%s], sessionId [%s] and customerId [%s]",
			resUserId, resSessionId, resCustomerId)

		resp.SessionId = resSessionId
		resp.CustomerId = resCustomerId
	} else {
		newSessionId, resultUserId, resCustomerId, ok, errStr := updateUserDataWithSessionId(userInfo, true, lc)
		if !ok {
			anlogger.Errorf(lc, "internal_start.go : return %s to client", errStr)
			return apimodel.AuthResp{}, nil
		}
		//we need this coz scope of resultUserId
		resUserId = resultUserId
		resp.SessionId = newSessionId
		resp.CustomerId = resCustomerId
		anlogger.Debugf(lc, "internal_start.go : userId [%s] for such phone [%s] was previously reserved, new sessionId [%s] was generated, customerId [%s]",
			resUserId, userInfo.Phone, resp.SessionId, resp.CustomerId)
		anlogger.Infof(lc, "internal_start.go : userId [%s] for such phone was previously reserved, new sessionId [%s] was generated, customerId [%s]",
			resUserId, resp.SessionId, resp.CustomerId)
	}
	//send analytics event
	event := apimodel.NewUserAcceptTermsEvent(&request, sourceIp, resUserId, resCustomerId, true, wasCreated)
	apimodel.SendAnalyticEvent(event, resUserId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	//send analytics event
	eventStartVerify := apimodel.NewUserVerificationStart(resUserId, userInfo.VerifyProvider, userInfo.Locale, userInfo.CountryCode)
	apimodel.SendAnalyticEvent(eventStartVerify, resUserId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	//send delayed task to check complete verification
	task := apimodel.NewCheckVerificationCompleteTask(userInfo.Phone, userTableName)
	apimodel.SendAsyncTask(task, asyncTaskQueue, userInfo.UserId, defaultMaxTimeToCompleteVerificationInSec, awsSqsClient, anlogger, lc)

	anlogger.Infof(lc, "internal_start.go : successfully initiate verification for userId [%s], return response=%v to the client", resUserId, resp)
	return resp, nil
}

//return generated userId, was everything ok and error string
func generateUserId(phone string, lc *lambdacontext.LambdaContext) (string, bool, string) {
	anlogger.Debugf(lc, "internal_start.go : generate userId for phone [%s]", phone)
	saltForUserId, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "internal_start.go : error while generate salt for userId, phone [%s] : %v", phone, err)
		return "", false, apimodel.InternalServerError
	}
	sha := sha1.New()
	_, err = sha.Write([]byte(phone))
	if err != nil {
		anlogger.Errorf(lc, "internal_start.go : error while write phone to sha algo, phone [%s] : %v", phone, err)
		return "", false, apimodel.InternalServerError
	}
	_, err = sha.Write([]byte(saltForUserId.String()))
	if err != nil {
		anlogger.Errorf(lc, "internal_start.go : error while write salt to sha algo, phone [%s] : %v", phone, err)
		return "", false, apimodel.InternalServerError
	}
	resultUserId := fmt.Sprintf("%x", sha.Sum(nil))
	anlogger.Debugf(lc, "internal_start.go : successfully generate userId [%s] for phone [%s]", resultUserId, phone)
	return resultUserId, true, ""
}

//return updated sessionId, userId, customerId is everything ok and error string
func updateUserDataWithSessionId(info *apimodel.UserInfo, isItAndroid bool, lc *lambdacontext.LambdaContext) (string, string, string, bool, string) {
	anlogger.Debugf(lc, "internal_start.go : update user data %v, is it Android %v", info, isItAndroid)

	deviceColumnName := apimodel.AndroidDeviceModelColumnName
	osColumnName := apimodel.AndroidOsVersionColumnName
	if !isItAndroid {
		deviceColumnName = apimodel.IOSDeviceModelColumnName
		osColumnName = apimodel.IOsVersionColumnName
	}

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#sessionId":        aws.String(apimodel.SessionIdColumnName),
				"#time":             aws.String(apimodel.UpdatedTimeColumnName),
				"#provider":         aws.String(apimodel.VerifyProviderColumnName),
				"#verifyStatus":     aws.String(apimodel.VerificationStatusColumnName),
				"#verifyStartAt":    aws.String(apimodel.VerificationStartAtColumnName),
				"#locale":           aws.String(apimodel.LocaleColumnName),
				"#currentIsAndroid": aws.String(apimodel.CurrentActiveDeviceIsAndroid),
				"#device":           aws.String(deviceColumnName),
				"#os":               aws.String(osColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":sV": {
					S: aws.String(fmt.Sprint(info.SessionId)),
				},
				":tV": {
					S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
				},
				":providerV": {
					S: aws.String(info.VerifyProvider),
				},
				":verifyStatusV": {
					S: aws.String("start"),
				},
				":verifyStartAtV": {
					N: aws.String(fmt.Sprintf("%v", time.Now().Unix())),
				},
				":localeV": {
					S: aws.String(info.Locale),
				},
				":currentIsAndroidV": {
					BOOL: aws.Bool(isItAndroid),
				},
				":deviceV": {
					S: aws.String(info.DeviceModel),
				},
				":osV": {
					S: aws.String(info.OsVersion),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				apimodel.PhoneColumnName: {
					S: aws.String(info.Phone),
				},
			},
			TableName:        aws.String(userTableName),
			UpdateExpression: aws.String("SET #sessionId = :sV, #time = :tV, #provider = :providerV, #verifyStatus = :verifyStatusV, #verifyStartAt = :verifyStartAtV, #locale = :localeV, #currentIsAndroid = :currentIsAndroidV, #device = :deviceV, #os = :osV"),
			ReturnValues:     aws.String("ALL_NEW"),
		}

	res, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "internal_start.go : error while update user data %v : %v", info, err)
		return "", "", "", false, apimodel.InternalServerError
	}

	resultSessionId := *res.Attributes[apimodel.SessionIdColumnName].S
	resultUserId := *res.Attributes[apimodel.UserIdColumnName].S
	resultCustomerId := *res.Attributes[apimodel.CustomerIdColumnName].S

	anlogger.Debugf(lc, "internal_start.go : successfully update user data %v, is it Android %v", info, isItAndroid)

	return resultSessionId, resultUserId, resultCustomerId, true, ""
}

//return ok and error string
func updateRequestId(phone, requestId, userId string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "internal_start.go : update request id [%s], for phone [%s] for userId [%s]", requestId, phone, userId)
	if len(requestId) == 0 {
		anlogger.Debugf(lc, "internal_start.go : update request id with empty value not allowed, userId [%s], return", userId)
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
		anlogger.Errorf(lc, "internal_start.go : error while update request id [%s], for phone [%s] and userId [%s] : %v", requestId, phone, userId, err)
		return false, apimodel.InternalServerError
	}

	anlogger.Debugf(lc, "internal_start.go : successfully update request id [%s] for phone [%s] and userId [%s]", requestId, phone, userId)

	return true, ""
}

//return userId, sessionId,  was user created, was everything ok and error string
func createUserInfo(userInfo *apimodel.UserInfo, isItAndroid bool, lc *lambdacontext.LambdaContext) (userId, sessionId, customerId string, wasCreated, ok bool, errorStr string) {
	anlogger.Debugf(lc, "internal_start.go : reserve userId [%s] and customerId [%s], for phone [%s], is it Android [%v], userInfo=%v", userInfo.UserId, userInfo.CustomerId, userInfo.Phone, isItAndroid, userInfo)

	deviceColumnName := apimodel.AndroidDeviceModelColumnName
	osColumnName := apimodel.AndroidOsVersionColumnName
	if !isItAndroid {
		deviceColumnName = apimodel.IOSDeviceModelColumnName
		osColumnName = apimodel.IOsVersionColumnName
	}

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#userId":    aws.String(apimodel.UserIdColumnName),
				"#sessionId": aws.String(apimodel.SessionIdColumnName),

				"#countryCode":      aws.String(apimodel.CountryCodeColumnName),
				"#phoneNumber":      aws.String(apimodel.PhoneNumberColumnName),
				"#time":             aws.String(apimodel.UpdatedTimeColumnName),
				"#customerId":       aws.String(apimodel.CustomerIdColumnName),
				"#provider":         aws.String(apimodel.VerifyProviderColumnName),
				"#verifyStatus":     aws.String(apimodel.VerificationStatusColumnName),
				"#verifyStartAt":    aws.String(apimodel.VerificationStartAtColumnName),
				"#locale":           aws.String(apimodel.LocaleColumnName),
				"#currentIsAndroid": aws.String(apimodel.CurrentActiveDeviceIsAndroid),
				"#device":           aws.String(deviceColumnName),
				"#os":               aws.String(osColumnName),
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
				":localeV": {
					S: aws.String(userInfo.Locale),
				},
				":currentIsAndroidV": {
					BOOL: aws.Bool(isItAndroid),
				},
				":deviceV": {
					S: aws.String(userInfo.DeviceModel),
				},
				":osV": {
					S: aws.String(userInfo.OsVersion),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				apimodel.PhoneColumnName: {
					S: aws.String(userInfo.Phone),
				},
			},
			ConditionExpression: aws.String(fmt.Sprintf("attribute_not_exists(%v)", apimodel.PhoneColumnName)),

			TableName:        aws.String(userTableName),
			UpdateExpression: aws.String("SET #userId = :uV, #sessionId = :sV, #countryCode = :cV, #phoneNumber = :pnV, #time = :tV, #customerId = :cIdV, #provider = :providerV, #verifyStatus = :verifyStatusV, #verifyStartAt = :verifyStartAtV, #locale = :localeV, #currentIsAndroid = :currentIsAndroidV, #device = :deviceV, #os = :osV"),
			ReturnValues:     aws.String("ALL_NEW"),
		}

	res, err := awsDbClient.UpdateItem(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				anlogger.Debugf(lc, "internal_start.go : userId [%s] was already reserved for this phone number, phone [%s]", userInfo.UserId, userInfo.Phone)
				return "", "", "", false, true, ""
			}
		}
		anlogger.Errorf(lc, "internal_start.go : error while reserve userId [%s] and customerId [%s] for phone [%s], userInfo=%v : %v", userInfo.UserId, userInfo.CustomerId, userInfo.Phone, userInfo, err)
		return "", "", "", false, false, apimodel.InternalServerError
	}

	resUserId := *res.Attributes[apimodel.UserIdColumnName].S
	resSessionId := *res.Attributes[apimodel.SessionIdColumnName].S
	resCustomerId := *res.Attributes[apimodel.CustomerIdColumnName].S
	anlogger.Debugf(lc, "internal_start.go : successfully reserve userId [%s] and customerId [%s] for phone [%s], is it android [%v]", resUserId, resCustomerId, userInfo.Phone, isItAndroid)
	return resUserId, resSessionId, resCustomerId, true, true, ""
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.StartReq, bool) {
	var req apimodel.StartReq
	err := json.Unmarshal([]byte(params), &req)

	if err != nil {
		anlogger.Errorf(lc, "internal_start.go : error parsing required params from the body string [%s] : %v", params, err)
		return nil, false
	}

	if req.CountryCallingCode == 0 || req.Phone == "" || req.DateTimeTermsAndConditions == 0 ||
		req.DateTimePrivacyNotes == 0 || req.DateTimeLegalAge == 0 || req.Locale == "" ||
		req.DeviceModel == "" || req.OsVersion == "" {
		anlogger.Errorf(lc, "internal_start.go : one of the required param is nil, req %v", req)
		return nil, false
	}

	anlogger.Debugf(lc, "internal_start.go : successfully parse param string [%s] to request=%v", params, req)
	return &req, true
}

func main() {
	basicLambda.Start(handler)
}