package apimodel

import (
	"../sys_log"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"encoding/json"
	"github.com/aws/aws-sdk-go/service/firehose"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

//return userId, was everything ok, error string in case of error
func FindUserId(accessToken, userProfileTableName string, awsDbClient *dynamodb.DynamoDB,
	anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) (string, bool, string) {
	anlogger.Debugf(lc, "common_action.go : start find user by accessToken [%s]", accessToken)

	if len(accessToken) == 0 {
		anlogger.Errorf(lc, "common_action.go : empty access token")
		return "", false, WrongRequestParamsClientError
	}

	input := &dynamodb.QueryInput{
		ExpressionAttributeNames: map[string]*string{
			"#token": aws.String(AccessTokenColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":tV": {
				S: aws.String(accessToken),
			},
		},
		KeyConditionExpression: aws.String("#token = :tV"),
		TableName:              aws.String(userProfileTableName),
		IndexName:              aws.String(AccessTokenGSIName),
	}

	result, err := awsDbClient.Query(input)
	if err != nil {
		anlogger.Errorf(lc, "common_action.go : error while find user by accessToken [%s] : %v", accessToken, err)
		return "", false, InternalServerError
	}

	if len(result.Items) == 0 {
		anlogger.Warnf(lc, "common_action.go : warning, there is no user with such accessToken [%s]", accessToken)
		return "", false, InvalidAccessTokenClientError
	}

	if len(result.Items) > 1 {
		anlogger.Errorf(lc, "common_action.go : error, there more than one user with such accessToken [%s]", accessToken)
		return "", false, InternalServerError
	}

	userId := *result.Items[0][UserIdColumnName].S
	anlogger.Debugf(lc, "common_action.go : successfully fetched userId [%s] by accessToken [%s]", userId, accessToken)

	return userId, true, ""
}

func SendAnalyticEvent(event interface{}, userId, deliveryStreamName string, awsDeliveryStreamClient *firehose.Firehose,
	anlogger *syslog.Logger, lc *lambdacontext.LambdaContext) {
	anlogger.Debugf(lc, "common_action.go : start sending analytics event [%v] for userId [%s]", event, userId)
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
