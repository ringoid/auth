package apimodel

type UserInfo struct {
	UserId    string
	SessionId string
	//full phone with country code included
	Phone       string
	CountryCode int
	PhoneNumber string
	CustomerId  string
}
