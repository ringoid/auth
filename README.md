# auth service

### Start auth

* stage url ``https://oka5pmgpb3.execute-api.eu-west-2.amazonaws.com/Prod/start``

POST request

Headers:

* Content-Type : application/json

Body:

    {
        "countryCallingCode":7,
        "phone":"9211234567",
        "device":"iPhone X",
        "os":"iOS",
        "screen":"bla-bla",
        "locale":"en"
    }`
    
    all parameters are required except locale
    
 Locale values could be found [here](https://www.twilio.com/docs/verify/supported-languages)
    
 Response Body:
 
    {
        "errorCode":"",
        "errorMessage":"",
        "sessionId":"sdfsdf-fsdf-fsd"
    }
    
Possible errorCodes:

* InternalServerError
* WrongRequestParamsClientError
* PhoneNumberClientError
* CountryCallingCodeClientError


## Analytics Events

1. USER_ACCEPT_TERMS

* userId - string
* device - string
* os - string
* screen - string
* sourceIp - string
* unixTime - int
* eventType - string (USER_ACCEPT_TERMS)
* locale - string

`{"userId":"aslkdl-asfmfa-asd","device":"iPhone X","os":"iOS","screen":"hd","sourceIp":"82.102.27.75","unixTime":1534338646,"locale":"","eventType":"USER_ACCEPT_TERMS"}`
