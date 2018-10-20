package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"../sys_log"
	"../apimodel"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"errors"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

var anlogger *syslog.Logger
var awsDbClient *dynamodb.DynamoDB
var awsCWClient *cloudwatch.CloudWatch

var baseCloudWatchNamespace string
var nexmoMetricName string
var twilioMetricName string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : internal_handle_task.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : internal_handle_task.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : internal_handle_task.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : internal_handle_task.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "internal-handle-task-auth"))
	if err != nil {
		fmt.Errorf("internal_handle_task.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_handle_task.go : logger was successfully initialized")

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : internal_handle_task.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_handle_task.go : aws session was successfully initialized")

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : internal_handle_task.go : dynamodb client was successfully initialized")

	awsCWClient = cloudwatch.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : internal_handle_task.go : cloudwatch client was successfully initialized")

	baseCloudWatchNamespace, ok = os.LookupEnv("BASE_CLOUD_WATCH_NAMESPACE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : internal_handle_task.go : env can not be empty BASE_CLOUD_WATCH_NAMESPACE")
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_handle_task.go : start with BASE_CLOUD_WATCH_NAMESPACE = [%s]", baseCloudWatchNamespace)

	nexmoMetricName, ok = os.LookupEnv("CLOUD_WATCH_NEXMO_NOT_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : internal_handle_task.go : env can not be empty CLOUD_WATCH_NEXMO_NOT_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME")
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_handle_task.go : start with CLOUD_WATCH_NEXMO_NOT_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME = [%s]", nexmoMetricName)

	twilioMetricName, ok = os.LookupEnv("CLOUD_WATCH_TWILIO_NOT_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : internal_handle_task.go : env can not be empty CLOUD_WATCH_TWILIO_NOT_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME")
	}
	anlogger.Debugf(nil, "lambda-initialization : internal_handle_task.go : start with CLOUD_WATCH_TWILIO_NOT_COMPLETE_VERIFICATION_IN_TIME_METRIC_NAME = [%s]", twilioMetricName)
}

func handler(ctx context.Context, event events.SQSEvent) (error) {
	lc, _ := lambdacontext.FromContext(ctx)

	anlogger.Debugf(lc, "internal_handle_task.go : start handle event %v", event)

	for _, record := range event.Records {
		anlogger.Debugf(lc, "internal_handle_task.go : handle record %v", record)
		body := record.Body
		var aTask apimodel.AsyncTask
		err := json.Unmarshal([]byte(body), &aTask)
		if err != nil {
			anlogger.Errorf(lc, "internal_handle_task.go : error unmarshal body [%s] to AsyncTask : %v", body, err)
			return errors.New(fmt.Sprintf("error unmarshal body %s : %v", body, err))
		}
		switch aTask.TaskType {
		case apimodel.AuthCheckVerificationCompeteTask:
			err = checkVerifyComplete([]byte(body), baseCloudWatchNamespace, nexmoMetricName, twilioMetricName, awsCWClient, awsDbClient, lc, anlogger)
			if err != nil {
				return err
			}
		default:
			return errors.New(fmt.Sprintf("unsuported taks type %s", aTask.TaskType))
		}
	}

	anlogger.Debugf(lc, "internal_handle_task.go : successfully complete task %v", event)
	return nil
}

func main() {
	basicLambda.Start(handler)
}
