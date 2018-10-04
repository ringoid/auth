package apimodel

import (
	"fmt"
)

const (
	Region     = "eu-west-1"
	MaxRetries = 3

	TwilioSecretKeyBase = "%s/Twilio/Api/Key"
	TwilioApiKeyName    = "twilio-api-key"

	NexmoSecretKeyBase = "%s/Nexmo/Api/Key"
	NexmoApiKeyName    = "nexmo-api-key"

	NexmoApiSecretKeyBase = "%s/Nexmo/Api/Secret"
	NexmoApiSecretKeyName = "nexmo-api-secret"

	SecretWordKeyName = "secret-word-key"
	SecretWordKeyBase = "%s/SecretWord"

	SessionGSIName = "sessionGSI"

	PhoneColumnName     = "phone"
	UserIdColumnName    = "user_id"
	SessionIdColumnName = "session_id"

	CountryCodeColumnName      = "country_code"
	PhoneNumberColumnName      = "phone_number"
	TokenUpdatedTimeColumnName = "token_updated_at"

	SessionTokenColumnName = "session_token"
	SexColumnName          = "sex"

	YearOfBirthColumnName         = "year_of_birth"
	ProfileCreatedAt              = "profile_created_at"
	CustomerIdColumnName          = "customer_id"
	VerifyProviderColumnName      = "verify_provider"
	VerifyRequestIdColumnName     = "verify_request_id"
	VerificationStatusColumnName  = "verify_status"
	VerificationStartAtColumnName = "verification_start_at"
	LocaleColumnName              = "locale"

	UpdatedTimeColumnName    = "updated_at"
	LastOnlineTimeColumnName = "last_online_time"

	WhoCanSeePhotoColumnName      = "who_can_see_photo"
	SafeDistanceInMeterColumnName = "safe_distance_in_meter"
	PushMessagesColumnName        = "push_messages"
	PushMatchesColumnName         = "push_matches"
	PushLikesColumnName           = "push_likes"
	InAppMessagesColumnName       = "in_app_messages"
	InAppMatchesColumnName        = "in_app_matches"
	InAppLikesColumnName          = "in_app_likes"

	AccessTokenUserIdClaim       = "userId"
	AccessTokenSessionTokenClaim = "sessionToken"

	InternalServerError           = `{"errorCode":"InternalServerError","errorMessage":"Internal Server Error"}`
	WrongRequestParamsClientError = `{"errorCode":"WrongParamsClientError","errorMessage":"Wrong request params"}`
	PhoneNumberClientError        = `{"errorCode":"PhoneNumberClientError","errorMessage":"Phone number is invalid"}`
	CountryCallingCodeClientError = `{"errorCode":"CountryCallingCodeClientError","errorMessage":"Country code is invalid"}`

	WrongSessionIdClientError        = `{"errorCode":"WrongSessionIdClientError","errorMessage":"Session id is invalid"}`
	NoPendingVerificationClientError = `{"errorCode":"NoPendingVerificationClientError","errorMessage":"No pending verifications found"}`
	WrongVerificationCodeClientError = `{"errorCode":"WrongVerificationCodeClientError","errorMessage":"Wrong verification code"}`

	WrongYearOfBirthClientError   = `{"errorCode":"WrongYearOfBirthClientError","errorMessage":"Wrong year of birth"}`
	WrongSexClientError           = `{"errorCode":"WrongSexClientError","errorMessage":"Wrong sex"}`
	InvalidAccessTokenClientError = `{"errorCode":"InvalidAccessTokenClientError","errorMessage":"Invalid access token"}`

	Twilio = "Twilio"
	Nexmo  = "Nexmo"
)

type BaseResponse struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func (resp BaseResponse) String() string {
	return fmt.Sprintf("[BaseResponse={errorCode=%s, errorMessage=%s}", resp.ErrorCode, resp.ErrorMessage)
}

var RoutingRuleMap map[int]string

func init() {
	RoutingRuleMap = make(map[int]string)
	RoutingRuleMap[1] = Twilio
	RoutingRuleMap[44] = Twilio
}
