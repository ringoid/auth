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

func GetSecret(secretBase, secretKeyName string, awsSession *session.Session, anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) string {
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
