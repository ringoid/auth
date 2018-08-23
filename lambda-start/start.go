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
	"github.com/aws/aws-sdk-go/aws/awserr"
)

var anlogger *syslog.Logger
var twilioKey string
var awsDbClient *dynamodb.DynamoDB
var userTableName string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string

const (
	region     = "eu-west-1"
	maxRetries = 3

	twilioApiKeyName    = "twilio-api-key"
	twilioSecretKeyBase = "%s/Twilio/Api/Key"

	phoneColumnName     = "phone"
	userIdColumnName    = "user_id"
	sessionIdColumnName = "session_id"

	countryCodeColumnName = "country_code"
	phoneNumberColumnName = "phone_number"
	updatedTimeColumnName = "updated_at"
)

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
	anlogger.Debugf("start.go : logger was successfully initialized")

	userTableName, ok = os.LookupEnv("USER_TABLE")
	if !ok {
		fmt.Printf("start.go : env can not be empty USER_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf("start.go : start with USER_TABLE = [%s]", userTableName)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(region).WithMaxRetries(maxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf("start.go : error during initialization : %v", err)
	}
	anlogger.Debugf("start.go : aws session was successfully initialized")

	twilioSecretKeyName = fmt.Sprintf(twilioSecretKeyBase, env)
	svc := secretsmanager.New(awsSession)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(twilioSecretKeyName),
	}

	result, err := svc.GetSecretValue(input)
	if err != nil {
		anlogger.Fatalf("start.go : error reading [%s] secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	var secretMap map[string]string
	decoder := json.NewDecoder(strings.NewReader(*result.SecretString))
	err = decoder.Decode(&secretMap)
	if err != nil {
		anlogger.Fatalf("start.go : error decode [%s] secret from Secret Manager : %v", twilioSecretKeyName, err)
	}
	twilioKey, ok = secretMap[twilioApiKeyName]
	if !ok {
		anlogger.Fatalln("start.go : Twilio Api Key is empty")
	}
	anlogger.Debugf("start.go : Twilio Api Key was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf("start.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf("start.go : env can not be empty DELIVERY_STREAM")
		os.Exit(1)
	}
	anlogger.Debugf("start.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf("start.go : firehose client was successfully initialized")
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	anlogger.Debugf("start.go : handle request %v", request)

	reqParam, ok := parseParams(request.Body)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.WrongRequestParamsClientError}, nil
	}

	sourceIp := request.RequestContext.Identity.SourceIP

	userId, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf("start.go : error while generate userId : %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}

	sessionId, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf("start.go : error while generate sessionId : %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	fullPhone := strconv.Itoa(reqParam.CountryCallingCode) + reqParam.Phone
	userInfo := &apimodel.UserInfo{
		UserId:      userId.String(),
		SessionId:   sessionId.String(),
		Phone:       fullPhone,
		CountryCode: reqParam.CountryCallingCode,
		PhoneNumber: reqParam.Phone,
	}

	resUserId, resSessionId, wasCreated, ok, errStr := createUserInfo(userInfo)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
	}

	resp := apimodel.AuthResp{}
	if wasCreated {
		anlogger.Debugf("start.go : new user was created with userId %s and sessionId %s", resUserId, resSessionId)
		resp.SessionId = resSessionId
	} else {
		newSessionId, ok, errStr := updateSessionId(userInfo.Phone, userInfo.SessionId)
		if !ok {
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: errStr}, nil
		}
		resp.SessionId = newSessionId
		anlogger.Debugf("start.go : user with such phone %s already exist, new sessionId id was generated %s",
			userInfo.Phone, resp.SessionId)
	}
	//send analytics event
	event := apimodel.NewUserAcceptTermsEvent(*reqParam, sourceIp, resUserId)
	sendAnalyticEvent(event)

	//send sms
	ok, errorStr := startVerify(reqParam.CountryCallingCode, reqParam.Phone, reqParam.Locale)
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: errorStr}, nil
	}

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf("start.go : error while marshaling resp object : %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: apimodel.InternalServerError}, nil
	}
	//return OK with SessionId
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: string(body)}, nil
}

//return updated sessionId, is everything ok and error string
func updateSessionId(phone, sessionId string) (string, bool, string) {
	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#sessionId": aws.String(sessionIdColumnName),
				"#time":      aws.String(updatedTimeColumnName),
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
				phoneColumnName: {
					S: aws.String(phone),
				},
			},
			TableName:        aws.String(userTableName),
			UpdateExpression: aws.String("SET #sessionId = :sV, #time = :tV"),
			ReturnValues:     aws.String("ALL_NEW"),
		}

	res, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf("start.go : error while update sessionId : %v", err)
		return "", false, apimodel.InternalServerError
	}

	resSessionId := *res.Attributes[sessionIdColumnName].S
	return resSessionId, true, ""
}

//return userId, sessionId, was user created, was everything ok and error string
func createUserInfo(userInfo *apimodel.UserInfo) (string, string, bool, bool, string) {
	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#userId":    aws.String(userIdColumnName),
				"#sessionId": aws.String(sessionIdColumnName),

				"#countryCode": aws.String(countryCodeColumnName),
				"#phoneNumber": aws.String(phoneNumberColumnName),
				"#time":        aws.String(updatedTimeColumnName),
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
			},
			Key: map[string]*dynamodb.AttributeValue{
				phoneColumnName: {
					S: aws.String(userInfo.Phone),
				},
			},
			ConditionExpression: aws.String(fmt.Sprintf("attribute_not_exists(%v)", userIdColumnName)),

			TableName:        aws.String(userTableName),
			UpdateExpression: aws.String("SET #userId = :uV, #sessionId = :sV, #countryCode = :cV, #phoneNumber = :pnV, #time = :tV"),
			ReturnValues:     aws.String("ALL_NEW"),
		}

	res, err := awsDbClient.UpdateItem(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return "", "", false, true, ""
			}
		}
		anlogger.Errorf("start.go : error while creating user : %v", err)
		return "", "", false, false, apimodel.InternalServerError
	}

	resUserId := *res.Attributes[userIdColumnName].S
	resSessionId := *res.Attributes[sessionIdColumnName].S
	return resUserId, resSessionId, true, true, ""
}

func parseParams(params string) (*apimodel.StartReq, bool) {
	var req apimodel.StartReq
	err := json.Unmarshal([]byte(params), &req)

	if err != nil {
		anlogger.Errorf("start.go : error parsing required params from the string %s : %v", params, err)
		return nil, false
	}

	if req.CountryCallingCode == 0 || req.Phone == "" || req.DateTimeTermsAndConditions == "" ||
		req.DateTimePrivacyNotes == "" || req.DateTimeLegalAge == "" {
		anlogger.Errorf("start.go : one of the required param is nil, req %v", req)
		return nil, false
	}

	return &req, true
}

func sendAnalyticEvent(event interface{}) {
	data, err := json.Marshal(event)
	if err != nil {
		anlogger.Errorf("start.go : error marshaling analytics event : %v", err)
		return
	}
	newLine := "\n"
	data = append(data, newLine...)
	_, err = awsDeliveryStreamClient.PutRecord(&firehose.PutRecordInput{
		DeliveryStreamName: aws.String(deliveryStreamName),
		Record: &firehose.Record{
			Data: data,
		},
	})

	if err != nil {
		anlogger.Errorf("start.go : error sending analytics event : %v", err)
	}
}

func startVerify(code int, number, locale string) (bool, string) {
	params := fmt.Sprintf("via=sms&&phone_number=%s&&country_code=%d", number, code)
	if len(locale) != 0 {
		params = fmt.Sprintf("via=sms&&phone_number=%s&&country_code=%d&&locale=%s", number, code, locale)
	}
	url := "https://api.authy.com/protected/json/phones/verification/start"

	req, err := http.NewRequest("POST", url, strings.NewReader(params))

	if err != nil {
		anlogger.Errorf("start.go : error while construct the request : %v", err)
		return false, apimodel.InternalServerError
	}

	req.Header.Set("X-Authy-API-Key", twilioKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}

	anlogger.Debugf("start.go : make POST request by url %s with params %s", url, params)

	resp, err := client.Do(req)
	if err != nil {
		anlogger.Errorf("start.go error while making request : %v", err)
		return false, apimodel.InternalServerError
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		anlogger.Errorf("start.go : error reading response body from Twilio : %v", err)
		return false, apimodel.InternalServerError
	}

	if resp.StatusCode != 200 {
		anlogger.Errorf("start.go : error while sending sms, status %v, body %v",
			resp.StatusCode, string(body))

		var errorResp map[string]interface{}
		err := json.Unmarshal(body, &errorResp)
		if err != nil {
			anlogger.Errorf("start.go : error parsing Twilio response : %v", err)
			return false, apimodel.InternalServerError
		}

		if errorCodeObject, ok := errorResp["error_code"]; ok {
			if errorCodeStr, ok := errorCodeObject.(string); ok {
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

	anlogger.Infof("start.go : sms was successfully sent, status %v, body %v",
		resp.StatusCode, string(body))
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
