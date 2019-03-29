package apimodel

func NewSettings(req *CreateReq) *Settings {
	defaultLocale := "en"
	if req.AppSettings.Locale != "" {
		defaultLocale = req.AppSettings.Locale
	}
	return &Settings{
		Locale:   defaultLocale,
		TimeZone: req.AppSettings.TimeZone,
		Push:     req.AppSettings.Push,
	}
}
