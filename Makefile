all: clean stage-deploy

build:
	@echo '--- Building start-auth function ---'
	GOOS=linux go build lambda-start/start.go
	@echo '--- Building complete-auth function ---'
	GOOS=linux go build lambda-complete/complete.go
	@echo '--- Building create-profile-auth function ---'
	GOOS=linux go build lambda-create/create.go

zip_lambda: build
	@echo '--- Zip start-auth function ---'
	zip start-auth.zip ./start
	@echo '--- Zip complete-auth function ---'
	zip complete-auth.zip ./complete
	@echo '--- Zip create-profile-auth function ---'
	zip create-auth.zip ./create

stage-deploy: zip_lambda
	@echo '--- Build lambda stage ---'
	@echo 'Package template'
	sam package --template-file auth-template.yaml --s3-bucket ringoid-cloudformation-template --output-template-file auth-template-packaged.yaml
	@echo 'Deploy stage-auth-stack'
	sam deploy --template-file auth-template-packaged.yaml --s3-bucket ringoid-cloudformation-template --stack-name stage-auth-stack --capabilities CAPABILITY_IAM --parameter-overrides Env=stage --no-fail-on-empty-changeset

clean:
	@echo '--- Delete old artifacts ---'
	rm -rf auth-template-packaged.yaml
	rm -rf start
	rm -rf start-auth.zip
	rm -rf complete
	rm -rf complete-auth.zip
	rm -rf create-auth.zip
	rm -rf create

