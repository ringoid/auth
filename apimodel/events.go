package apimodel

import "time"

type UserAcceptTermsEvent struct {
	UserId    string `json:"userId"`
	Device    string `json:"device"`
	Os        string `json:"os"`
	Screen    string `json:"screen"`
	Locale    string `json:"locale"`
	SourceIp  string `json:"sourceIp"`
	UnixTime  int64  `json:"unixTime"`
	EventType string `json:"eventType"`
}

func NewUserAcceptTermsEvent(req StartReq, sourceIp, userId string) UserAcceptTermsEvent {
	return UserAcceptTermsEvent{
		UserId: userId,
		Device: req.Device,
		Os:     req.Os,
		Screen: req.Screen,
		Locale: req.Locale,
		//gdpr?
		SourceIp: sourceIp,

		UnixTime:  time.Now().Unix(),
		EventType: "USER_ACCEPT_TERMS",
	}
}
