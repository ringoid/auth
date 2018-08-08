package main

import (
	"context"
	basicLambda "github.com/aws/aws-lambda-go/lambda"
	"log"
)

func handler(ctx context.Context, request interface{}) (string, error) {
	log.Printf("Request : %v", request)
	return "", nil
}

func main() {
	basicLambda.Start(handler)
}
