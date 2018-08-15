package apimodel

import "time"

type UserAcceptTermsEvent struct {
	StartReq
	SourceIp  string `json:"sourceIp"`
	UnixTime  int64  `json:"unixTime"`
	EventType string `json:"eventType"`
}

func NewUserAcceptTermsEvent(req StartReq, sourceIp string) UserAcceptTermsEvent {
	return UserAcceptTermsEvent{
		StartReq:  req,
		SourceIp:  sourceIp,
		UnixTime:  time.Now().Unix(),
		EventType: "USER_ACCEPT_TERMS",
	}
}
