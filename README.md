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
