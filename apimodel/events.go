package apimodel

import "time"

type UserAcceptTermsEvent struct {
	UserId                     string `json:"userId"`
	Locale                     string `json:"locale"`
	SourceIp                   string `json:"sourceIp"`
	ClientValidationFail       bool   `json:"clientValidationFail"`
	UnixTime                   int64  `json:"unixTime"`
	EventType                  string `json:"eventType"`
	DateTimeTermsAndConditions string `json:"dtTC"`
	DateTimePrivacyNotes       string `json:"dtPN"`
	DateTimeLegalAge           string `json:"dtLA"`
}

func NewUserAcceptTermsEvent(req StartReq, sourceIp, userId string) UserAcceptTermsEvent {
	return UserAcceptTermsEvent{
		UserId: userId,
		Locale: req.Locale,
		//gdpr?
		SourceIp: sourceIp,

		ClientValidationFail:       req.ClientValidationFail,
		UnixTime:                   time.Now().Unix(),
		DateTimeLegalAge:           req.DateTimeLegalAge,
		DateTimePrivacyNotes:       req.DateTimePrivacyNotes,
		DateTimeTermsAndConditions: req.DateTimeTermsAndConditions,

		EventType: "AUTH_USER_ACCEPT_TERMS",
	}
}

type UserVerificationCompleteEvent struct {
	UserId    string `json:"userId"`
	UnixTime  int64  `json:"unixTime"`
	EventType string `json:"eventType"`
}

func NewUserVerificationCompleteEvent(userId string) UserVerificationCompleteEvent {
	return UserVerificationCompleteEvent{
		UserId:    userId,
		UnixTime:  time.Now().Unix(),
		EventType: "AUTH_USER_COMPLETE_VERIFICATION",
	}
}

type UserProfileCreatedEvent struct {
	UserId      string `json:"userId"`
	Sex         string `json:"sex"`
	YearOfBirth int    `json:"yearOfBirth"`
	UnixTime    int64  `json:"unixTime"`
	EventType   string `json:"eventType"`
}

func NewUserProfileCreatedEvent(userId string, req CreateReq) UserProfileCreatedEvent {
	return UserProfileCreatedEvent{
		UserId:      userId,
		Sex:         req.Sex,
		YearOfBirth: req.YearOfBirth,
		UnixTime:    time.Now().Unix(),
		EventType:   "AUTH_USER_PROFILE_CREATED",
	}
}
