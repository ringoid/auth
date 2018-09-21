package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"../sys_log"
	"../apimodel"
	"github.com/aws/aws-sdk-go/aws"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/lambda"
	"strings"
)

var anlogger *syslog.Logger
var clientLambda *lambda.Lambda
var allLambdaNames string

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("warm_up_auth.go : env can not be empty ENV")
		os.Exit(1)
	}
	fmt.Printf("warm_up_auth.go : start with ENV = [%s]", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("warm_up_auth.go : env can not be empty PAPERTRAIL_LOG_ADDRESS")
		os.Exit(1)
	}
	fmt.Printf("warm_up_auth.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]", papertrailAddress)

	anlogger, err = syslog.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "warm-up-auth"))
	if err != nil {
		fmt.Errorf("warm_up_auth.go : error during startup : %v", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "warm_up_auth.go : logger was successfully initialized")

	allLambdaNames, ok = os.LookupEnv("NEED_WARM_UP_LAMBDA_NAMES")
	if !ok {
		fmt.Printf("warm_up_auth.go : env can not be empty NEED_WARM_UP_LAMBDA_NAMES")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "warm_up_auth.go : start with NEED_WARM_UP_LAMBDA_NAMES = [%s]", allLambdaNames)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(apimodel.Region).WithMaxRetries(apimodel.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "warm_up_auth.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "warm_up_auth.go : aws session was successfully initialized")

	clientLambda = lambda.New(awsSession)
	anlogger.Debugf(nil, "warm_up_auth.go : lambda client was successfully initialized")
}

func handler(ctx context.Context, request events.CloudWatchEvent) error {
	lc, _ := lambdacontext.FromContext(ctx)
	names := strings.Split(allLambdaNames, ",")
	for _, n := range names {
		apimodel.WarmUpLambda(n, clientLambda, anlogger, lc)
	}
	return nil
}

func main() {
	basicLambda.Start(handler)
}
