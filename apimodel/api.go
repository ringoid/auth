package apimodel

const (
	InternalServerError           = `{"errorCode":"InternalServerError","errorMessage":"Internal Server Error"}`
	WrongRequestParamsClientError = `{"errorCode":"WrongParamsClientError","errorMessage":"Wrong request params"}`
	PhoneNumberClientError        = `{"errorCode":"PhoneNumberClientError","errorMessage":"Phone number is invalid"}`
	CountryCallingCodeClientError = `{"errorCode":"CountryCallingCodeClientError","errorMessage":"Country code is invalid"}`

	WrongSessionIdClientError        = `{"errorCode":"WrongSessionIdClientError","errorMessage":"Session id is invalid"}`
	NoPendingVerificationClientError = `{"errorCode":"NoPendingVerificationClientError","errorMessage":"No pending verifications found"}`
	WrongVerificationCodeClientError = `{"errorCode":"WrongVerificationCodeClientError","errorMessage":"Wrong verification code"}`
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
