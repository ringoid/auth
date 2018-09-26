package apimodel

import (
	"../sys_log"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"encoding/json"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/dgrijalva/jwt-go"
	"fmt"
	"strings"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-lambda-go/events"
	"time"
)

//return userId, sessionToken, ok, error string
func DecodeToken(token, secretWord string, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (string, string, bool, string) {

	receiveToken, err := jwt.Parse(token, func(rt *jwt.Token) (interface{}, error) {
		if _, ok := rt.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", rt.Header["alg"])
		}
		return []byte(secretWord), nil
	})
	if err != nil {
		anlogger.Warnf(lc, "common_action.go : error parse access token [%s] : %v", token, err)
		return "", "", false, InternalServerError
	}

	if claims, ok := receiveToken.Claims.(jwt.MapClaims); ok && receiveToken.Valid {
		userId := fmt.Sprintf("%v", claims[AccessTokenUserIdClaim])
		sessionToken := fmt.Sprintf("%v", claims[AccessTokenSessionTokenClaim])
		anlogger.Debugf(lc, "common_action.go : successfully parse access token, userId [%s], sessionToken [%s]", userId, sessionToken)
		return userId, sessionToken, true, ""
	} else {
		anlogger.Warnf(lc, "common_action.go : access token [%s] is not valid", token)
		return "", "", false, InternalServerError
	}
}

//return is session valid, ok, error string
func IsSessionValid(userId, sessionToken, userProfileTableName string, awsDbClient *dynamodb.DynamoDB,
	anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (bool, bool, string) {

	anlogger.Debugf(lc, "common_action.go : check that sessionToken [%s] is valid for userId [%s]", sessionToken, userId)
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(userProfileTableName),
	}

	result, err := awsDbClient.GetItem(input)
	if err != nil {
		anlogger.Errorf(lc, "common_action.go : error getting userInfo for userId [%s] : %v", userId, err)
		return false, false, InternalServerError
	}

	if len(result.Item) == 0 {
		anlogger.Warnf(lc, "common_action.go : there is no user with such userId [%s], sessionToken [%s]", userId, sessionToken)
		return false, true, ""
	}

	lastSessionToken := *result.Item[SessionTokenColumnName].S
	if sessionToken != lastSessionToken {
		anlogger.Warnf(lc, "common_action.go : sessionToken [%s] expired for userId [%s]", sessionToken, userId)
		return false, true, ""
	}

	anlogger.Debugf(lc, "common_action.go : session token is valid for userId [%s]", userId)
	return true, true, ""
}

func SendAnalyticEvent(event interface{}, userId, deliveryStreamName string, awsDeliveryStreamClient *firehose.Firehose,
	anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) {
	anlogger.Debugf(lc, "common_action.go : send analytics event [%v] for userId [%s]", event, userId)
	data, err := json.Marshal(event)
	if err != nil {
		anlogger.Errorf(lc, "common_action.go : error marshaling analytics event [%v] for userId [%s] : %v", event, userId, err)
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
		anlogger.Errorf(lc, "common_action.go : error sending analytics event [%v] for userId [%s] : %v", event, userId, err)
	}

	anlogger.Debugf(lc, "common_action.go : successfully send analytics event [%v] for userId [%s]", event, userId)
}

//ok and error string
func SendCommonEvent(event interface{}, userId, commonStreamName string, awsKinesisClient *kinesis.Kinesis,
	anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "common_action.go : send common event [%v] for userId [%s]", event, userId)
	data, err := json.Marshal(event)
	if err != nil {
		anlogger.Errorf(lc, "common_action.go : error marshaling common event [%v] for userId [%s] : %v", event, userId, err)
		return false, InternalServerError
	}
	input := &kinesis.PutRecordInput{
		StreamName:   aws.String(commonStreamName),
		PartitionKey: aws.String(userId),
		Data:         []byte(data),
	}
	_, err = awsKinesisClient.PutRecord(input)
	if err != nil {
		anlogger.Errorf(lc, "common_action.go : error putting common event into stream, event [%v] for userId [%s] : %v", event, userId, err)
		return false, InternalServerError
	}
	anlogger.Debugf(lc, "common_action.go : successfully send common event [%v] for userId [%s]", event, userId)
	return true, ""
}

func GetSecret(secretBase, secretKeyName string, awsSession *session.Session, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) string {
	anlogger.Debugf(lc, "common_action.go : read secret with secretBase [%s], secretKeyName [%s]", secretBase, secretKeyName)
	svc := secretsmanager.New(awsSession)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretBase),
	}

	result, err := svc.GetSecretValue(input)
	if err != nil {
		anlogger.Fatalf(lc, "common_action.go : error reading %s secret from Secret Manager : %v", secretBase, err)
	}
	var secretMap map[string]string
	decoder := json.NewDecoder(strings.NewReader(*result.SecretString))
	err = decoder.Decode(&secretMap)
	if err != nil {
		anlogger.Fatalf(lc, "common_action.go : error decode %s secret from Secret Manager : %v", secretBase, err)
	}
	secret, ok := secretMap[secretKeyName]
	if !ok {
		anlogger.Fatalf(lc, "common_action.go : secret %s is empty", secretBase)
	}
	anlogger.Debugf(lc, "common_action.go : secret %s was successfully initialized", secretBase)

	return secret
}

func WarmUpLambda(functionName string, clientLambda *lambda.Lambda, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) {
	anlogger.Debugf(lc, "common_action.go : warmup lambda [%s]", functionName)
	req := WarmUpRequest{
		WarmUpRequest: true,
	}
	jsonBody, err := json.Marshal(req)
	if err != nil {
		anlogger.Errorf(lc, "common_action.go : error marshaling req %v into json : %v", req, err)
		return
	}

	apiReq := events.APIGatewayProxyRequest{
		Body: string(jsonBody),
	}

	apiJsonBody, err := json.Marshal(apiReq)
	if err != nil {
		anlogger.Errorf(lc, "common_action.go : error marshaling req %v into json : %v", apiReq, err)
		return
	}

	payload := apiJsonBody
	if strings.Contains(functionName, "internal") {
		payload = jsonBody
	}
	_, err = clientLambda.Invoke(&lambda.InvokeInput{FunctionName: aws.String(functionName), Payload: payload, InvocationType: aws.String("Event")})

	if err != nil {
		anlogger.Errorf(lc, "warm_up.go : error invoke function [%s] with body %s : %v", functionName, string(payload), err)
		return
	}

	anlogger.Debugf(lc, "common_action.go : successfully warmup lambda [%s]", functionName)
	return
}

func IsItWarmUpRequest(body string, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) bool {
	anlogger.Debugf(lc, "common_action.go : is it warm up request, body [%s]", body)
	var req WarmUpRequest
	err := json.Unmarshal([]byte(body), &req)

	if err != nil {
		anlogger.Errorf(lc, "common_action.go : error unmarshal required params from the string %s : %v", body, err)
		return false
	}
	result := req.WarmUpRequest
	anlogger.Debugf(lc, "common_action.go : successfully check that it's warm up request, body [%s], result [%v]", body, result)
	return result
}

//return ok and error string
func UpdateLastOnlineTime(userId, userProfileTableName string, awsDbClient *dynamodb.DynamoDB, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "common_action.go : update last online time for userId [%s]", userId)

	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#onlineTime": aws.String(LastOnlineTimeColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":onlineTimeV": {
					N: aws.String(fmt.Sprintf("%v", time.Now().Unix())),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				UserIdColumnName: {
					S: aws.String(userId),
				},
			},
			TableName:        aws.String(userProfileTableName),
			UpdateExpression: aws.String("SET #onlineTime = :onlineTimeV"),
		}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		anlogger.Errorf(lc, "common_action.go : error while update last online time for userId [%s] : %v", userId, err)
		return false, InternalServerError
	}

	anlogger.Debugf(lc, "common_action.go : successfully update last online time for userId [%s]", userId)

	return true, ""
}

//return userId, ok and error string
func Login(token, secretWord, userProfileTable, commonStreamName string, awsDbClient *dynamodb.DynamoDB, awsKinesisClient *kinesis.Kinesis,
	anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (string, bool, string) {

	anlogger.Debugf(lc, "common_action.go : login for token [%s]", token)

	userId, sessionToken, ok, errStr := DecodeToken(token, secretWord, anlogger, lc)
	if !ok {
		return "", ok, errStr
	}

	valid, ok, errStr := IsSessionValid(userId, sessionToken, userProfileTable, awsDbClient, anlogger, lc)
	if !ok {
		return "", ok, errStr
	}

	if !valid {
		return "", false, InvalidAccessTokenClientError
	}

	ok, errStr = UpdateLastOnlineTime(userId, userProfileTable, awsDbClient, anlogger, lc)
	if !ok {
		return "", ok, errStr
	}

	event := NewUserOnlineEvent(userId)
	ok, errStr = SendCommonEvent(event, userId, commonStreamName, awsKinesisClient, anlogger, lc)
	if !ok {
		return "", ok, errStr
	}

	anlogger.Debugf(lc, "common_action.go : successfully login for token [%s]", token)
	return userId, true, ""
}
