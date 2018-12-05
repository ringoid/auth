stage-all: clean stage-deploy
test-all: clean test-deploy
prod-all: clean prod-deploy

build:
	go get -u github.com/ringoid/commons
	@echo '--- Building create-profile-auth function ---'
	GOOS=linux go build lambda-create/create.go
	@echo '--- Building internal-get-user-id-auth function ---'
	GOOS=linux go build lambda-internal-getuserid/internal_get_user_id.go
	@echo '--- Building update-settings-auth function ---'
	GOOS=linux go build lambda-update-settings/update_settings.go
	@echo '--- Building get-settings-auth function ---'
	GOOS=linux go build lambda-get-settings/get_settings.go
	@echo '--- Building warm-up-auth function ---'
	GOOS=linux go build lambda-warmup/warm_up.go
	@echo '--- Building internal-clean-db-auth function ---'
	GOOS=linux go build lambda-clean-db/clean.go
	@echo '--- Building lambda-delete-user-auth function ---'
	GOOS=linux go build lambda-delete-user/delete.go
	@echo '--- Building lambda-handle-stream-image function ---'
	GOOS=linux go build lambda-handle-stream/handle_stream.go lambda-handle-stream/block.go


zip_lambda: build
	@echo '--- Zip create-profile-auth function ---'
	zip create-auth.zip ./create
	@echo '--- Zip internal-getuserid-auth function ---'
	zip internal-getuserid-auth.zip ./internal_get_user_id
	@echo '--- Zip update-settings-auth function ---'
	zip update-settings-auth.zip ./update_settings
	@echo '--- Zip get-settings-auth function ---'
	zip get-settings-auth.zip ./get_settings
	@echo '--- Zip warm-up-auth function ---'
	zip warmup-auth.zip ./warm_up
	@echo '--- Zip internal-clean-db-auth function ---'
	zip clean.zip ./clean
	@echo '--- Zip delete-user-auth function ---'
	zip delete.zip ./delete
	@echo '--- Zip lambda-handle-stream-image function ---'
	zip handle_stream.zip ./handle_stream

test-deploy: zip_lambda
	@echo '--- Build lambda test ---'
	@echo 'Package template'
	sam package --template-file cf/test/auth-template.yaml --s3-bucket ringoid-cloudformation-template --output-template-file auth-template-packaged.yaml
	@echo 'Deploy test-auth-stack'
	sam deploy --template-file auth-template-packaged.yaml --s3-bucket ringoid-cloudformation-template --stack-name test-auth-stack --capabilities CAPABILITY_IAM --parameter-overrides Env=test --no-fail-on-empty-changeset

stage-deploy: zip_lambda
	@echo '--- Build lambda stage ---'
	@echo 'Package template'
	sam package --template-file cf/auth-template.yaml --s3-bucket ringoid-cloudformation-template --output-template-file auth-template-packaged.yaml
	@echo 'Deploy stage-auth-stack'
	sam deploy --template-file auth-template-packaged.yaml --s3-bucket ringoid-cloudformation-template --stack-name stage-auth-stack --capabilities CAPABILITY_IAM --parameter-overrides Env=stage --no-fail-on-empty-changeset

prod-deploy: zip_lambda
	@echo '--- Build lambda prod ---'
	@echo 'Package template'
	sam package --template-file cf/auth-template.yaml --s3-bucket ringoid-cloudformation-template --output-template-file auth-template-packaged.yaml
	@echo 'Deploy prod-auth-stack'
	sam deploy --template-file auth-template-packaged.yaml --s3-bucket ringoid-cloudformation-template --stack-name prod-auth-stack --capabilities CAPABILITY_IAM --parameter-overrides Env=prod --no-fail-on-empty-changeset

clean:
	@echo '--- Delete old artifacts ---'
	rm -rf auth-template-packaged.yaml
	rm -rf create-auth.zip
	rm -rf create
	rm -rf internal_get_user_id
	rm -rf internal-getuserid-auth.zip
	rm -rf update-settings-auth.zip
	rm -rf update_settings
	rm -rf get-settings-auth.zip
	rm -rf get_settings
	rm -rf warmup-auth.zip
	rm -rf warm_up
	rm -rf clean.zip
	rm -rf clean
	rm -rf delete.zip
	rm -rf delete
	rm -rf handle_stream
	rm -rf handle_stream.zip

