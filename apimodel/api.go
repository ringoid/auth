package apimodel

const (
	InternalServerError           = `{"errorCode":"InternalServerError","errorMessage":"Internal Server Error"}`
	WrongRequestParamsClientError = `{"errorCode":"WrongParamsClientError","errorMessage":"Wrong request params"}`
	PhoneNumberClientError        = `{"errorCode":"PhoneNumberClientError","errorMessage":"Phone number is invalid"}`
	CountryCallingCodeClientError = `{"errorCode":"CountryCallingCodeClientError","errorMessage":"Country code is invalid"}`
)

//Request - Response model
type AuthResp struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	SessionId    string `json:"sessionId"`
}

type StartReq struct {
	CountryCallingCode int    `json:"countryCallingCode"`
	Phone              string `json:"phone"`
	Device             string `json:"device"`
	Os                 string `json:"os"`
	Screen             string `json:"screen"`
	Locale             string `json:"locale"`
}
