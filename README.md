# auth service

### Start auth

* stage url ``https://oka5pmgpb3.execute-api.eu-west-2.amazonaws.com/Prod/start``

POST request

Headers:

* Content-Type : application/json

Body:

    {
        "countryCode":7,
        "phone":"9211234567",
        "device":"iPhone X",
        "os":"iOS",
        "screen":"bla-bla"
    }`
    
    all parameters are required 
    
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


## Analytics Events

1. USER_ACCEPT_TERMS

* countryCode - int
* phone - string
* device - string
* os - string
* screen - string
* sourceIp - string
* unixTime - int
* eventType - string (USER_ACCEPT_TERMS)

`{"countryCode":7,"phone":"9211112233","device":"iPhone X","os":"iOS","screen":"hd","sourceIp":"82.102.27.75","unixTime":1534338646,"eventType":"USER_ACCEPT_TERMS"}`
