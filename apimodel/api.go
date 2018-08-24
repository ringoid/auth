package apimodel

const (
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
)

type BaseResponse struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

//Request - Response model
type AuthResp struct {
	BaseResponse
	SessionId string `json:"sessionId"`
}

type StartReq struct {
	CountryCallingCode         int    `json:"countryCallingCode"`
	Phone                      string `json:"phone"`
	ClientValidationFail       bool   `json:"clientValidationFail"`
	Locale                     string `json:"locale"`
	DateTimeTermsAndConditions string `json:"dtTC"`
	DateTimePrivacyNotes       string `json:"dtPN"`
	DateTimeLegalAge           string `json:"dtLA"`
}

type VerifyReq struct {
	SessionId        string `json:"sessionId"`
	VerificationCode int    `json:"verificationCode"`
}

type VerifyResp struct {
	BaseResponse
	AccessToken         string `json:"accessToken"`
	AccountAlreadyExist bool   `json:"accountAlreadyExist"`
}

type CreateReq struct {
	AccessToken string `json:"accessToken"`
	YearOfBirth int    `json:"yearOfBirth"`
	Sex         string `json:"sex"`
}
