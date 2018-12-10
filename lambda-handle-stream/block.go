package main

import (
	"github.com/aws/aws-lambda-go/lambdacontext"
	"fmt"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/ringoid/commons"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
)

func block(body []byte, userProfileTable string,
	awsDbClient *dynamodb.DynamoDB, lc *lambdacontext.LambdaContext, anlogger *commons.Logger) error {

	var aEvent commons.UserBlockOtherEvent
	err := json.Unmarshal([]byte(body), &aEvent)
	if err != nil {
		anlogger.Errorf(lc, "block.go : error unmarshal body [%s] to UserBlockOtherEvent: %v", string(body), err)
		return errors.New(fmt.Sprintf("error unmarshal body %s : %v", string(body), err))
	}

	anlogger.Debugf(lc, "block.go : handle block event %v", aEvent)

	ok, errStr := markUserAsPartOfReport(aEvent.TargetUserId, userProfileTable, awsDbClient, lc, anlogger)
	if !ok {
		return errors.New(errStr)
	}
	ok, errStr = markUserAsPartOfReport(aEvent.UserId, userProfileTable, awsDbClient, lc, anlogger)
	if !ok {
		return errors.New(errStr)
	}

	anlogger.Debugf(lc, "block.go : successfully handle block event %v", aEvent)
	return nil
}

//return ok and error string
func markUserAsPartOfReport(userId, userProfileTable string, awsDbClient *dynamodb.DynamoDB, lc *lambdacontext.LambdaContext, anlogger *commons.Logger) (bool, string) {
	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#reportStatus": aws.String(commons.UserReportStatusColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":reportStatusV": {
				S: aws.String(commons.UserTakePartInReport),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		ConditionExpression: aws.String(fmt.Sprintf("attribute_exists(%v)", commons.UserIdColumnName)),
		TableName:           aws.String(userProfileTable),
		UpdateExpression:    aws.String("SET #reportStatus = :reportStatusV"),
	}

	_, err := awsDbClient.UpdateItem(input)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				anlogger.Warnf(lc, "block.go : warning when mark user like take part in report, user with userId [%s] doesn't exist", userId)
				return true, ""
			}
		}
		anlogger.Warnf(lc, "block.go : error mark user with userId [%s] like take part in report : %v", userId, err)
		return false, commons.InternalServerError
	}

	anlogger.Infof(lc, "block.go : successfully mark user userId [%s] like take part in report", userId)

	return true, ""
}
