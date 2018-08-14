package apimodel

type AuthResp struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	SessionId  string `json:"sessionId"`
}

const (
	InternalServerError           = `{"errorCode":"InternalServerError","errorMessage":"Internal Server Error"}`
	WrongRequestParamsClientError = `{"errorCode":"WrongParamsClientError","errorMessage":"Wrong request params"}`
	PhoneNumberClientError        = `{"errorCode":"PhoneNumberClientError","errorMessage":"Phone number is invalid"}`
)

type StartReq struct {
	CountryCode int    `json:"countryCode"`
	Phone       string `json:"phone"`
	Device      string `json:"device"`
	Os          string `json:"os"`
	Screen      string `json:"screen"`
}
