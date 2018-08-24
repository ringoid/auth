# auth service

### STAGE API ENDPOINT IS ``lewwnhue55.execute-api.eu-west-1.amazonaws.com``
### PROD API ENDPOINT IS ````

### Start auth

* url ``https://{API ENDPOINT}/Prod/start_verification``

POST request

Headers:

* Content-Type : application/json

Body:

    {
        "countryCallingCode":7,
        "phone":"9211234567",
        "dtTC":"2018-08-01 12:34:54 UTC+3", //date and time when Terms and Conditions were accepted
        "dtLA":"2018-08-01 12:34:55 UTC+3", //date and time when Privacy Notes were accepted
        "dtPN":"2018-08-01 12:34:56 UTC+3", //date and time when Legal age was confirmed
        "locale":"en",
        "clientValidationFail":true
    }
    
    all parameters are required except locale
    
 Locale values could be found [here](https://www.twilio.com/docs/verify/supported-languages)
    
 Response Body:
 
    {
        "errorCode":"",
        "errorMessage":"",
        "sessionId":"sdfsdf-fsdf-fsd",
        "customerId":"ksjdhfha-asff"
    }
    
Possible errorCodes:

* InternalServerError
* WrongRequestParamsClientError
* PhoneNumberClientError
* CountryCallingCodeClientError

### Complete auth

* url ``https://{API ENDPOINT}/Prod/complete_verification``

POST request

Headers:

* Content-Type : application/json

Body:

    {
        "sessionId":"sdkjfhh-dfsdf-e333",
        "verificationCode":"6121"
    }
    
    all parameters are required
    
 Response Body:
 
    {
        "accessToken":"aslkdjflkjh-sdfasdfsadf-dd",
        "accountAlreadyExist":false,
        "errorCode":"",
        "errorMessage":""
    }
    
Possible errorCodes:

* InternalServerError
* WrongRequestParamsClientError
* WrongSessionIdClientError
* NoPendingVerificationClientError
* WrongVerificationCodeClientError

### Create user profile

* url ``https://{API ENDPOINT}/Prod/create_profile``

POST request

Headers:

* Content-Type : application/json

Body:

    {
        "accessToken":"adasdasd-fadfs-sdffd",
        "yearOfBirth":1982,
        "sex":"male" // possible values are **male** or **female** 
    }
    
    all parameters are required
    
 Response Body:
 
    {
        "errorCode":"",
        "errorMessage":""
    }
    
Possible errorCodes:

* InternalServerError
* WrongYearOfBirthClientError
* WrongSexClientError
* WrongRequestParamsClientError
* InvalidAccessTokenClientError

## Analytics Events

1. AUTH_USER_ACCEPT_TERMS

* userId - string
* sourceIp - string
* unixTime - int
* eventType - string (AUTH_USER_ACCEPT_TERMS)
* locale - string
* clientValidationFail - was phone number's validation failed on client side
* dtTC - date and time when Terms and conditions were accepted
* dtPN - date and time when Privacy Notes were accepted
* dtLA - date and time when Legal age was confirmed

`{"userId":"aslkdl-asfmfa-asd","sourceIp":"82.102.27.75","unixTime":1534338646,"dtTC":"2018-08-01 12:34:54 UTC+3","dtPN":"2018-08-01 12:34:54 UTC+3","dtLA":"2018-08-01 12:34:54 UTC+3",locale":"","clientValidationFail":true,"eventType":"AUTH_USER_ACCEPT_TERMS"}`

2. AUTH_USER_COMPLETE_VERIFICATION

* userId - string
* unixTime - int
* eventType - string (AUTH_USER_COMPLETE_VERIFICATION)

`{"userId":"aslkdl-asfmfa-asd","unixTime":1534338646,"eventType":"AUTH_USER_COMPLETE_VERIFICATION"}`

3. AUTH_USER_PROFILE_CREATED

* userId - string
* sex - string
* yearOfBirth - int
* unixTime - int
* eventType - string (AUTH_USER_PROFILE_CREATED)

`{"userId":"aslkdl-asfmfa-asd","sex":"male","yearOfBirth":"1982","unixTime":1534338646,"eventType":"AUTH_USER_PROFILE_CREATED"}`