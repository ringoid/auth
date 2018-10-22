package main

import (
	"github.com/aws/aws-lambda-go/lambdacontext"
	"../sys_log"
	"../apimodel"
	"fmt"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

func checkVerifyComplete(body []byte, baseCloudWatchNamespace, nexmoMetricName, twilioMetricName string,
	cwClient *cloudwatch.CloudWatch, awsDbClient *dynamodb.DynamoDB, lc *lambdacontext.LambdaContext, anlogger *syslog.Logger) error {
	anlogger.Debugf(lc, "check_verify_complete.go : check complete verification for body %s", string(body))
	var rTask apimodel.CheckVerificationCompleteTask
	err := json.Unmarshal([]byte(body), &rTask)
	if err != nil {
		anlogger.Errorf(lc, "check_verify_complete.go : error unmarshal body [%s] to CheckVerificationCompleteTask: %v", body, err)
		return errors.New(fmt.Sprintf("error unmarshal body %s : %v", body, err))
	}

	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			apimodel.PhoneColumnName: {
				S: aws.String(rTask.Phone),
			},
		},
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(rTask.TableName),
	}

	result, err := awsDbClient.GetItem(input)
	if err != nil {
		anlogger.Errorf(lc, "check_verify_complete.go : error get item with phone [%s] from table name [%s] : %v", rTask.Phone, rTask.TableName, err)
		return errors.New(fmt.Sprintf("error get item with phone [%s] : %v", rTask.Phone, err))
	}

	if len(result.Item) == 0 {
		anlogger.Warnf(lc, "check_verify_complete.go : there is no item with phone [%s] in the table [%s]", rTask.Phone, rTask.TableName)
		return nil
	}

	status := *result.Item[apimodel.VerificationStatusColumnName].S
	provider := *result.Item[apimodel.VerifyProviderColumnName].S

	name := twilioMetricName
	if provider == "Nexmo" {
		name = nexmoMetricName
	}
	switch status {
	case "start":
		anlogger.Warnf(lc, "check_verify_complete.go : verification is still in progress for phone [%s] and provider [%s]", rTask.Phone, provider)
		apimodel.SendCloudWatchMetric(baseCloudWatchNamespace, name, 1, cwClient, anlogger, lc)
	case "complete":
		anlogger.Debugf(lc, "check_verify_complete.go : successfully complete verification in time for phone [%s] and provider [%s]", rTask.Phone, provider)
	default:
		anlogger.Errorf(lc, "check_verify_complete.go : unsupported status value [%s]", status)
		return errors.New(fmt.Sprintf("unsupported status value [%s]", status))
	}
	anlogger.Debugf(lc, "check_verify_complete.go : successfully check complete verification for body %s", string(body))
	return nil
}
