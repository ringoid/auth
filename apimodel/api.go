package apimodel

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
