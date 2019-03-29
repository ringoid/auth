package main

import (
	"github.com/ringoid/commons"
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"../apimodel"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/firehose"
	"os"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"time"
	"strconv"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/kinesis"
	"crypto/sha1"
	"github.com/satori/go.uuid"
	"github.com/dgrijalva/jwt-go"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"strings"
)

var anlogger *commons.Logger
var awsDbClient *dynamodb.DynamoDB
var userProfileTable string
var userSettingsTable string
var awsDeliveryStreamClient *firehose.Firehose
var deliveryStreamName string
var secretWord string
var commonStreamName string
var awsKinesisClient *kinesis.Kinesis

var baseCloudWatchNamespace string
var newUserWasCreatedMetricName string
var awsCWClient *cloudwatch.CloudWatch

func init() {
	var env string
	var ok bool
	var papertrailAddress string
	var err error
	var awsSession *session.Session

	env, ok = os.LookupEnv("ENV")
	if !ok {
		fmt.Printf("lambda-initialization : create.go : env can not be empty ENV\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : create.go : start with ENV = [%s]\n", env)

	papertrailAddress, ok = os.LookupEnv("PAPERTRAIL_LOG_ADDRESS")
	if !ok {
		fmt.Printf("lambda-initialization : create.go : env can not be empty PAPERTRAIL_LOG_ADDRESS\n")
		os.Exit(1)
	}
	fmt.Printf("lambda-initialization : create.go : start with PAPERTRAIL_LOG_ADDRESS = [%s]\n", papertrailAddress)

	anlogger, err = commons.New(papertrailAddress, fmt.Sprintf("%s-%s", env, "create-auth"))
	if err != nil {
		fmt.Errorf("lambda-initialization : create.go : error during startup : %v\n", err)
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : create.go : logger was successfully initialized")

	userProfileTable, ok = os.LookupEnv("USER_PROFILE_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : create.go : env can not be empty USER_PROFILE_TABLE")
		os.Exit(1)
	}
	anlogger.Debugf(nil, "lambda-initialization : create.go : start with USER_PROFILE_TABLE = [%s]", userProfileTable)

	userSettingsTable, ok = os.LookupEnv("USER_SETTINGS_TABLE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : create.go : env can not be empty USER_SETTINGS_TABLE")
	}
	anlogger.Debugf(nil, "lambda-initialization : create.go : start with USER_SETTINGS_TABLE = [%s]", userSettingsTable)

	baseCloudWatchNamespace, ok = os.LookupEnv("BASE_CLOUD_WATCH_NAMESPACE")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : create.go : env can not be empty BASE_CLOUD_WATCH_NAMESPACE")
	}
	anlogger.Debugf(nil, "lambda-initialization : create.go : start with BASE_CLOUD_WATCH_NAMESPACE = [%s]", baseCloudWatchNamespace)

	newUserWasCreatedMetricName, ok = os.LookupEnv("CLOUD_WATCH_NEW_USER_WAS_CREATED")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : create.go : env can not be empty CLOUD_WATCH_NEW_USER_WAS_CREATED")
	}
	anlogger.Debugf(nil, "lambda-initialization : create.go : start with CLOUD_WATCH_NEW_USER_WAS_CREATED = [%s]", newUserWasCreatedMetricName)

	awsSession, err = session.NewSession(aws.NewConfig().
		WithRegion(commons.Region).WithMaxRetries(commons.MaxRetries).
		WithLogger(aws.LoggerFunc(func(args ...interface{}) { anlogger.AwsLog(args) })).WithLogLevel(aws.LogOff))
	if err != nil {
		anlogger.Fatalf(nil, "lambda-initialization : create.go : error during initialization : %v", err)
	}
	anlogger.Debugf(nil, "lambda-initialization : create.go : aws session was successfully initialized")

	secretWord = commons.GetSecret(fmt.Sprintf(commons.SecretWordKeyBase, env), commons.SecretWordKeyName, awsSession, anlogger, nil)

	awsDbClient = dynamodb.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : create.go : dynamodb client was successfully initialized")

	deliveryStreamName, ok = os.LookupEnv("DELIVERY_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : create.go : env can not be empty DELIVERY_STREAM")
	}
	anlogger.Debugf(nil, "lambda-initialization : create.go : start with DELIVERY_STREAM = [%s]", deliveryStreamName)

	commonStreamName, ok = os.LookupEnv("COMMON_STREAM")
	if !ok {
		anlogger.Fatalf(nil, "lambda-initialization : create.go : env can not be empty COMMON_STREAM")
	}
	anlogger.Debugf(nil, "lambda-initialization : create.go : start with COMMON_STREAM = [%s]", commonStreamName)

	awsDeliveryStreamClient = firehose.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : create.go : firehose client was successfully initialized")

	awsKinesisClient = kinesis.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : create.go : kinesis client was successfully initialized")

	awsCWClient = cloudwatch.New(awsSession)
	anlogger.Debugf(nil, "lambda-initialization : create.go : cloudwatch client was successfully initialized")
}

func handler(ctx context.Context, request events.ALBTargetGroupRequest) (events.ALBTargetGroupResponse, error) {
	lc, _ := lambdacontext.FromContext(ctx)

	userAgent := request.Headers["user-agent"]
	if strings.HasPrefix(userAgent, "ELB-HealthChecker") {
		return commons.NewServiceResponse("{}"), nil
	}

	if request.HTTPMethod != "POST" {
		return commons.NewWrongHttpMethodServiceResponse(), nil
	}
	sourceIp := request.Headers["x-forwarded-for"]

	anlogger.Debugf(lc, "create.go : start handle request %v", request)

	appVersion, isItAndroid, ok, errStr := commons.ParseAppVersionFromHeaders(request.Headers, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "create.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	switch isItAndroid {
	case true:
		if appVersion < commons.MinimalAndroidBuildNum {
			errStr = commons.TooOldAppVersionClientError
			anlogger.Errorf(lc, "create.go : return %s to client", errStr)
			return commons.NewServiceResponse(errStr), nil
		}
	default:
		if appVersion < commons.MinimaliOSBuildNum {
			errStr = commons.TooOldAppVersionClientError
			anlogger.Errorf(lc, "create.go : return %s to client", errStr)
			return commons.NewServiceResponse(errStr), nil

		}
	}

	reqParam, ok, errStr := parseParams(request.Body, lc)
	if !ok {
		anlogger.Errorf(lc, "create.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	userId, ok, errStr := generateUserId(sourceIp, lc)
	if !ok {
		anlogger.Errorf(lc, "create.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	sessionId, err := uuid.NewV4()
	if err != nil {
		errStr := commons.InternalServerError
		anlogger.Errorf(lc, "create.go : error while generate sessionId for userId [%s] : %v", userId, err)
		anlogger.Errorf(lc, "create.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	customerId, err := uuid.NewV4()
	if err != nil {
		errStr := commons.InternalServerError
		anlogger.Errorf(lc, "create.go : error while generate customerId : %v", err)
		anlogger.Errorf(lc, "create.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = createUserProfile(userId, sessionId.String(), customerId.String(), appVersion, isItAndroid, reqParam, lc)
	if !ok {
		anlogger.Errorf(lc, "commons.go : return %s to client", errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	userSettings := apimodel.NewSettings(reqParam)
	ok, errStr = createUserSettingsIntoDynamo(userId, userSettings, lc)
	if !ok {
		anlogger.Errorf(lc, "create.go : userId [%s], customerId [%s], return %s to client", userId, customerId, errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	//send analytics events
	eventAcceptTerms := commons.NewUserAcceptTermsEvent(userId, customerId.String(), sourceIp,
		reqParam.DeviceModel, reqParam.OsVersion,
		reqParam.DateTimeLegalAge, reqParam.DateTimePrivacyNotes, reqParam.DateTimeTermsAndConditions,
		isItAndroid)
	commons.SendAnalyticEvent(eventAcceptTerms, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	eventNewUser := commons.NewUserProfileCreatedEvent(userId, reqParam.Sex, sourceIp, reqParam.ReferralId, reqParam.PrivateKey, reqParam.YearOfBirth)
	commons.SendAnalyticEvent(eventNewUser, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	settingsEvent := commons.NewUserSettingsUpdatedEvent(userId, sourceIp, userSettings.Locale, true, userSettings.Push, true, userSettings.TimeZone, true)
	commons.SendAnalyticEvent(settingsEvent, userId, deliveryStreamName, awsDeliveryStreamClient, anlogger, lc)

	//send common events
	partitionKey := userId
	ok, errStr = commons.SendCommonEvent(eventNewUser, userId, commonStreamName, partitionKey, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "create.go : userId [%s], customerId [%s], return %s to client", userId, customerId, errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	ok, errStr = commons.SendCommonEvent(settingsEvent, userId, commonStreamName, partitionKey, awsKinesisClient, anlogger, lc)
	if !ok {
		anlogger.Errorf(lc, "create.go : userId [%s], customerId [%s], return %s to client", userId, customerId, errStr)
		return commons.NewServiceResponse(errStr), nil
	}

	//send cloudwatch metric
	commons.SendCloudWatchMetric(baseCloudWatchNamespace, newUserWasCreatedMetricName, 1, awsCWClient, anlogger, lc)

	//create access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		commons.AccessTokenUserIdClaim:       userId,
		commons.AccessTokenSessionTokenClaim: sessionId.String(),
	})

	tokenToString, err := accessToken.SignedString([]byte(secretWord))
	if err != nil {
		errStr = commons.InternalServerError
		anlogger.Errorf(lc, "create.go : error sign the token for userId [%s], customerId [%s], return %s to the client : %v", userId, customerId, errStr, err)
		return commons.NewServiceResponse(errStr), nil
	}

	resp := apimodel.CreateResp{
		AccessToken: tokenToString,
		CustomerId:  customerId.String(),
	}

	body, err := json.Marshal(resp)
	if err != nil {
		anlogger.Errorf(lc, "create.go : error while marshaling resp object for userId [%s], customerId [%s] : %v", userId, customerId, err)
		anlogger.Errorf(lc, "create.go : userId [%s], customerId [%s], return %s to client", userId, customerId, commons.InternalServerError)
		return commons.NewServiceResponse(commons.InternalServerError), nil
	}
	anlogger.Infof(lc, "create.go : successfully create user and return access token for userId [%s], customerId [%s], sex [%s]",
		userId, customerId, reqParam.Sex)
	return commons.NewServiceResponse(string(body)), nil
}

func parseParams(params string, lc *lambdacontext.LambdaContext) (*apimodel.CreateReq, bool, string) {
	anlogger.Debugf(lc, "create.go : parse request body %s", params)
	var req apimodel.CreateReq
	err := json.Unmarshal([]byte(params), &req)
	if err != nil {
		anlogger.Errorf(lc, "create.go : error marshaling required params from the string [%s] : %v", params, err)
		return nil, false, commons.InternalServerError
	}

	if req.YearOfBirth < time.Now().UTC().Year()-150 || req.YearOfBirth > time.Now().UTC().Year()-18 {
		anlogger.Errorf(lc, "create.go : wrong year of birth [%d] request param, req %v", req.YearOfBirth, req)
		return nil, false, commons.WrongYearOfBirthClientError
	}

	if req.Sex == "" || (req.Sex != "male" && req.Sex != "female") {
		anlogger.Errorf(lc, "create.go : wrong sex [%s] request param, req %v", req.Sex, req)
		return nil, false, commons.WrongSexClientError
	}

	if req.DateTimeTermsAndConditions <= 0 ||
		req.DateTimePrivacyNotes <= 0 || req.DateTimeLegalAge <= 0 ||
		req.DeviceModel == "" || req.OsVersion == "" {
		anlogger.Errorf(lc, "create.go : one of the required param is nil, req %v", req)
		return nil, false, commons.WrongRequestParamsClientError
	}

	if req.ReferralId == "" {
		req.ReferralId = "n/a"
	} else if code := commons.ReferralCodes[req.ReferralId]; !code {
		anlogger.Errorf(lc, "create.go : unsupported referral id [%s]", req.ReferralId)
		return nil, false, commons.WrongRequestParamsClientError
	}

	if req.PrivateKey == "" && req.ReferralId == "n/a" {
		req.PrivateKey = "n/a"
	}

	if req.ReferralId != "n/a" && req.PrivateKey == "" {
		anlogger.Errorf(lc, "create.go : empty private key while referral id is [%s]", req.ReferralId)
		return nil, false, commons.WrongRequestParamsClientError
	}

	anlogger.Debugf(lc, "create.go : successfully parse request string [%s] to %v", params, req)
	return &req, true, ""
}

//ok (only if such userId doesn't exist), errorString if not ok
func createUserProfile(userId, sessionToken, customerId string, buildNum int, isItAndroid bool, req *apimodel.CreateReq, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "create.go : create user userId [%s], sessionToken [%s], customerId [%s], buildNum [%d], isItAndroid [%v] for request [%s]",
		userId, sessionToken, customerId, buildNum, isItAndroid, req)

	deviceColumnName := commons.AndroidDeviceModelColumnName
	osColumnName := commons.AndroidOsVersionColumnName
	buildNumColumnName := commons.CurrentAndroidBuildNum
	if !isItAndroid {
		buildNumColumnName = commons.CurrentiOSBuildNum
		deviceColumnName = commons.IOSDeviceModelColumnName
		osColumnName = commons.IOsVersionColumnName
	}

	input := &dynamodb.UpdateItemInput{
		ExpressionAttributeNames: map[string]*string{
			"#token":            aws.String(commons.SessionTokenColumnName),
			"#updatedAt":        aws.String(commons.TokenUpdatedTimeColumnName),
			"#sex":              aws.String(commons.SexColumnName),
			"#year":             aws.String(commons.YearOfBirthColumnName),
			"#created":          aws.String(commons.ProfileCreatedAt),
			"#onlineTime":       aws.String(commons.LastOnlineTimeColumnName),
			"#customerId":       aws.String(commons.CustomerIdColumnName),
			"#currentIsAndroid": aws.String(commons.CurrentActiveDeviceIsAndroid),
			"#buildNum":         aws.String(buildNumColumnName),
			"#device":           aws.String(deviceColumnName),
			"#os":               aws.String(osColumnName),
			"#status":           aws.String(commons.UserStatusColumnName),
			"#reportStatus":     aws.String(commons.UserReportStatusColumnName),
			"#referralId":       aws.String(commons.ReferralIdColumnName),
			"#privateKey":       aws.String(commons.PrivateKeyColumnName),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":tV": {
				S: aws.String(sessionToken),
			},
			":uV": {
				S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
			},
			":sV": {
				S: aws.String(req.Sex),
			},
			":yV": {
				N: aws.String(strconv.Itoa(req.YearOfBirth)),
			},
			":cV": {
				S: aws.String(time.Now().UTC().Format("2006-01-02-15-04-05.000")),
			},
			":onlineTimeV": {
				N: aws.String(fmt.Sprintf("%v", commons.UnixTimeInMillis())),
			},
			":buildNumV": {
				N: aws.String(strconv.Itoa(buildNum)),
			},
			":cIdV": {
				S: aws.String(customerId),
			},
			":currentIsAndroidV": {
				BOOL: aws.Bool(isItAndroid),
			},
			":deviceV": {
				S: aws.String(req.DeviceModel),
			},
			":osV": {
				S: aws.String(req.OsVersion),
			},
			":statusV": {
				S: aws.String(commons.UserActiveStatus),
			},
			":reportStatusV": {
				S: aws.String(commons.UserCleanReportStatus),
			},
			":referralIdV": {
				S: aws.String(req.ReferralId),
			},
			":privateKeyV": {
				S: aws.String(req.PrivateKey),
			},
		},
		Key: map[string]*dynamodb.AttributeValue{
			commons.UserIdColumnName: {
				S: aws.String(userId),
			},
		},
		ConditionExpression: aws.String(fmt.Sprintf("attribute_not_exists(%v)", commons.UserIdColumnName)),
		TableName:           aws.String(userProfileTable),
		UpdateExpression:    aws.String("SET #token = :tV, #updatedAt = :uV, #sex = :sV, #year = :yV, #created = :cV, #onlineTime = :onlineTimeV, #buildNum = :buildNumV, #customerId = :cIdV, #currentIsAndroid = :currentIsAndroidV, #device = :deviceV, #os = :osV, #status = :statusV, #reportStatus = :reportStatusV, #referralId = :referralIdV, #privateKey = :privateKeyV"),
	}

	_, err := awsDbClient.UpdateItem(input)

	if err != nil {
		anlogger.Errorf(lc, "create.go : error create user for userId [%s] : %v", userId, err)
		return false, commons.InternalServerError
	}

	anlogger.Debugf(lc, "create.go : successfully create user userId [%s], customerId [%s], buildNum [%d], isItAndroid [%v] for request [%s]",
		userId, customerId, buildNum, isItAndroid, req)

	return true, ""
}

//return generated userId, was everything ok and error string
func generateUserId(base string, lc *lambdacontext.LambdaContext) (string, bool, string) {
	anlogger.Debugf(lc, "create.go : generate userId for base string [%s]", base)
	saltForUserId, err := uuid.NewV4()
	if err != nil {
		anlogger.Errorf(lc, "create.go : error while generate salt for userId, base string [%s] : %v", base, err)
		return "", false, commons.InternalServerError
	}
	sha := sha1.New()
	_, err = sha.Write([]byte(base))
	if err != nil {
		anlogger.Errorf(lc, "create.go : error while write base string to sha algo, base string [%s] : %v", base, err)
		return "", false, commons.InternalServerError
	}
	_, err = sha.Write([]byte(saltForUserId.String()))
	if err != nil {
		anlogger.Errorf(lc, "create.go : error while write salt to sha algo, base string [%s] : %v", base, err)
		return "", false, commons.InternalServerError
	}
	resultUserId := fmt.Sprintf("%x", sha.Sum(nil))
	anlogger.Debugf(lc, "create.go : successfully generate userId [%s] for base string [%s]", resultUserId, base)
	return resultUserId, true, ""
}

//return ok and error string
func createUserSettingsIntoDynamo(userId string, settings *apimodel.Settings, lc *lambdacontext.LambdaContext) (bool, string) {
	anlogger.Debugf(lc, "create.go : create default user settings for userId [%s], settings=%v", userId, settings)
	input :=
		&dynamodb.UpdateItemInput{
			ExpressionAttributeNames: map[string]*string{
				"#locale":   aws.String(commons.LocaleColumnName),
				"#push":     aws.String(commons.PushColumnName),
				"#timeZone": aws.String(commons.TimeZoneColumnName),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":localeV": {
					S: aws.String(settings.Locale),
				},
				":pushV": {
					BOOL: aws.Bool(settings.Push),
				},
				":timeZoneV": {
					N: aws.String(strconv.Itoa(settings.TimeZone)),
				},
			},
			Key: map[string]*dynamodb.AttributeValue{
				commons.UserIdColumnName: {
					S: aws.String(userId),
				},
			},
			ConditionExpression: aws.String(fmt.Sprintf("attribute_not_exists(%v)", commons.UserIdColumnName)),
			TableName:           aws.String(userSettingsTable),
			UpdateExpression:    aws.String("SET #locale = :localeV, #push = :pushV, #timeZone = :timeZoneV"),
		}

	_, err := awsDbClient.UpdateItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				anlogger.Warnf(lc, "create.go : warning, default settings for userId [%s] already exist", userId)
				return true, ""
			}
		}
		anlogger.Errorf(lc, "create.go : error while creating default settings for userId [%s], settings=%v : %v", userId, settings, err)
		return false, commons.InternalServerError
	}

	anlogger.Infof(lc, "create.go : successfully create default user's settings for userId [%s]", userId)
	return true, ""
}

func main() {
	basicLambda.Start(handler)
}
