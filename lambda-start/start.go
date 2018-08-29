package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"net/http"
	"fmt"
	"io/ioutil"
	"strings"
	"os"
	"../sys_log"
	"github.com/aws/aws-lambda-go/events"
	"../apimodel"
	"encoding/json"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/satori/go.uuid"
	"time"
	"strconv"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

var anlogger *syslog.Logger
var twilioKey string
var awsDbClient *dynamodb.DynamoDB
var userTableName string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session
	var twilioSecretKeyName string

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

	twilioSecretKeyName = fmt.Sprintf(apimodel.TwilioSecretKeyBase, env)
	svc := secretsmanager.New(awsSession)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(twilioSecretKeyName),
	}

	result, err := svc.GetSecretValue(input)
	if err != nil {
		anlogger.Fatalf(nil, "start.go : error reading [%s] secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	var secretMap map[string]string
	decoder := json.NewDecoder(strings.NewReader(*result.SecretString))
	err = decoder.Decode(&secretMap)
	if err != nil {
		anlogger.Fatalf(nil, "start.go : error decode [%s] secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	twilioKey, ok = secretMap[apimodel.TwilioApiKeyName]
	if !ok {
		anlogger.Fatalln(nil, "start.go : Twilio Api Key is empty")
	}
	anlogger.Debugf(nil, "start.go : Twilio Api Key was successfully initialized")

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

	reqParam, ok := parseParams(request.Body, lc)
	if !ok {
		anlogger.Errorf(lc, "start.go : return %s to client", apimodel.WrongRequestParamsClientError)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.WrongRequestParamsClientError}, nil
	}

	sourceIp := request.RequestContext.Identity.SourceIP

	userId, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while generate userId : %v", err)
		anlogger.Errorf(lc, "start.go : return %s to client", apimodel.InternalServerError)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}

	sessionId, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while generate sessionId for userId [%s] : %v", userId, err)
		anlogger.Errorf(lc, "start.go : return %s to client", apimodel.InternalServerError)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	fullPhone := strconv.Itoa(reqParam.CountryCallingCode) + reqParam.Phone

	customerId, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while generate customerId : %v", err)
		anlogger.Errorf(lc, "start.go : return %s to client", apimodel.InternalServerError)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}

	userInfo := &apimodel.UserInfo{
		UserId:      userId.String(),
		SessionId:   sessionId.String(),
		Phone:       fullPhone,
		CountryCode: reqParam.CountryCallingCode,
		PhoneNumber: reqParam.Phone,
		CustomerId:  customerId.String(),
	}

	resUserId, resSessionId, resCustomerId, wasCreated, ok, errStr := createUserInfo(userInfo, lc)
	if !ok {
		anlogger.Errorf(lc, "start.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	resp := apimodel.AuthResp{}
	resp.CustomerId = resCustomerId
	if wasCreated {
		anlogger.Debugf(lc, "start.go : new userId was reserved, userId [%s] and sessionId [%s]", resUserId, resSessionId)
		resp.SessionId = resSessionId
	} else {
		newSessionId, ok, errStr := updateSessionId(userInfo.Phone, userInfo.SessionId, lc)
		if !ok {
			anlogger.Errorf(lc, "start.go : return %s to client", errStr)
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
		}
		resp.SessionId = newSessionId
		anlogger.Debugf(lc, "start.go : userId for such phone [%s] was previously reserved, new sessionId [%s] id was generated",
			userInfo.Phone, resp.SessionId)
	}
	//send analytics event
	event := apimodel.NewUserAcceptTermsEvent(reqParam, sourceIp, resUserId)
	apimodel.SendAnalyticEvent(event, resUserId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	//send sms
	ok, errorStr := startVerify(reqParam.CountryCallingCode, reqParam.Phone, reqParam.Locale, lc)
	if !ok {
		anlogger.Errorf(lc, "start.go : return %s to client", errStr)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errorStr}, nil
	}

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while marshaling resp object : %v", err)
		anlogger.Errorf(lc, "start.go : return %s to client", apimodel.InternalServerError)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	//return OK with SessionId
	anlogger.Debugf(lc, "start.go : return body=%s to the client", string(body))
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

//return updated sessionId, is everything ok and error string
func updateSessionId(phone, sessionId string, lc *lambdacontext.LambdaContext) (string, bool, string) {
	anlogger.Debugf(lc, "start.go : update sessionId [%s] for phone [%s]", sessionId, phone)

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#sessionId": aws.String(apimodel.SessionIdColumnName),
				"#time":      aws.String(apimodel.UpdatedTimeColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":sV": {
					S: aws.String(fmt.Sprint(sessionId)),
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
			UpdateExpression: aws.String("SET #sessionId = :sV, #time = :tV"),
			ReturnValues:     aws.String("ALL_NEW"),
		}

	res, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "start.go : error while update sessionId [%s] for phone [%s] : %v", sessionId, phone, err)
		return "", false, apimodel.InternalServerError
	}

	resSessionId := *res.Attributes[apimodel.SessionIdColumnName].S

	anlogger.Debugf(lc, "start.go : successfully update sessionId [%s] for phone [%s]", resSessionId, phone)

	return resSessionId, true, ""
}

//return userId, sessionId,  was user created, was everything ok and error string
func createUserInfo(userInfo *apimodel.UserInfo, lc *lambdacontext.LambdaContext) (userId, sessionId, customerId string, wasCreated, ok bool, errorStr string) {
	anlogger.Debugf(lc, "start.go : reserve userId and customerId, for phone [%s], userInfo=%v", userInfo.Phone, userInfo)

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#userId":    aws.String(apimodel.UserIdColumnName),
				"#sessionId": aws.String(apimodel.SessionIdColumnName),

				"#countryCode": aws.String(apimodel.CountryCodeColumnName),
				"#phoneNumber": aws.String(apimodel.PhoneNumberColumnName),
				"#time":        aws.String(apimodel.UpdatedTimeColumnName),
				"#customerId":  aws.String(apimodel.CustomerIdColumnName),
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
			},
			Key: map[string]*dynamodb.AttributeValue{
				apimodel.PhoneColumnName: {
					S: aws.String(userInfo.Phone),
				},
			},
			ConditionExpression: aws.String(fmt.Sprintf("attribute_not_exists(%v)", apimodel.UserIdColumnName)),

			TableName:        aws.String(userTableName),
			UpdateExpression: aws.String("SET #userId = :uV, #sessionId = :sV, #countryCode = :cV, #phoneNumber = :pnV, #time = :tV, #customerId = :cIdV"),
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

	return &req, true
}

func startVerify(code int, number, locale string, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "start.go : verify phone with code [%d] and phone [%s]", code, number)

	params := fmt.Sprintf("via=sms&&phone_number=%s&&country_code=%d", number, code)
	if len(locale) != 0 {
		params = fmt.Sprintf("via=sms&&phone_number=%s&&country_code=%d&&locale=%s", number, code, locale)
	}
	url := "https://api.authy.com/protected/json/phones/verification/start"

	req, err := http.NewRequest("POST", url, strings.NewReader(params))

	if err != nil {
		anlogger.Errorf(lc, "start.go : error while construct the request to verify code [%d] and phone [%s] : %v", code, number, err)
		return false, apimodel.InternalServerError
	}

	req.Header.Set("X-Authy-API-Key", twilioKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}

	anlogger.Debugf(lc, "start.go : make POST request by url %s with params %s", url, params)

	resp, err := client.Do(req)
	if err != nil {
		anlogger.Errorf(lc, "start.go error while making request by url %s with params %s : %v", url, params, err)
		return false, apimodel.InternalServerError
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		anlogger.Errorf(lc, "start.go : error reading response body from Twilio, code [%d] and phone [%s] : %v", code, number, err)
		return false, apimodel.InternalServerError
	}

	anlogger.Debugf(lc, "start.go : receive response from Twilio, body=%s, code [%d] and phone [%s]", string(body), code, number)
	if resp.StatusCode != 200 {
		anlogger.Errorf(lc, "start.go : error while sending sms, status %v, body %v",
			resp.StatusCode, string(body))

		var errorResp map[string]interface{}
		err := json.Unmarshal(body, &errorResp)
		if err != nil {
			anlogger.Errorf(lc, "start.go : error parsing Twilio response, code [%d] and phone [%s] : %v", code, number, err)
			return false, apimodel.InternalServerError
		}

		if errorCodeObject, ok := errorResp["error_code"]; ok {
			if errorCodeStr, ok := errorCodeObject.(string); ok {
				anlogger.Errorf(lc, "start.go : Twilio return error_code=%s, code [%d] and phone [%s] : %v", errorCodeStr, code, number, err)
				switch errorCodeStr {
				case "60033":
					return false, apimodel.PhoneNumberClientError
				case "60078":
					return false, apimodel.CountryCallingCodeClientError
				}
			}
		}

		return false, apimodel.InternalServerError
	}

	anlogger.Infof(lc, "start.go : sms was successfully sent, status %v, body %v, code [%d] and phone [%s]",
		resp.StatusCode, string(body), code, number)
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
