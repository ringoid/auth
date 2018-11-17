stage-all: clean stage-deploy
test-all: clean test-deploy
prod-all: clean prod-deploy

build:
	@echo '--- Building start-auth function ---'
	GOOS=linux go build lambda-start/start.go
	@echo '--- Building complete-auth function ---'
	GOOS=linux go build lambda-complete/complete.go
	@echo '--- Building create-profile-auth function ---'
	GOOS=linux go build lambda-create/create.go
	@echo '--- Building internal-get-user-id-auth function ---'
	GOOS=linux go build lambda-internal-getuserid/internal_get_user_id.go
	@echo '--- Building update-settings-auth function ---'
	GOOS=linux go build lambda-update-settings/update_settings.go
	@echo '--- Building get-settings-auth function ---'
	GOOS=linux go build lambda-get-settings/get_settings.go
	@echo '--- Building logout-auth function ---'
	GOOS=linux go build lambda-logout/logout.go
	@echo '--- Building warm-up-auth function ---'
	GOOS=linux go build lambda-warmup/warm_up.go
	@echo '--- Building lambda-handle-task-image function ---'
	GOOS=linux go build lambda-handle-task/internal_handle_task.go lambda-handle-task/check_verify_complete.go
	@echo '--- Building internal-start-auth function ---'
	GOOS=linux go build lambda-internal-start/internal_start.go
	@echo '--- Building internal-complete-auth function ---'
	GOOS=linux go build lambda-internal-complete/internal_complete.go
	@echo '--- Building internal-clean-db-auth function ---'
	GOOS=linux go build lambda-clean-db/clean.go

zip_lambda: build
	@echo '--- Zip start-auth function ---'
	zip start-auth.zip ./start
	@echo '--- Zip complete-auth function ---'
	zip complete-auth.zip ./complete
	@echo '--- Zip create-profile-auth function ---'
	zip create-auth.zip ./create
	@echo '--- Zip internal-getuserid-auth function ---'
	zip internal-getuserid-auth.zip ./internal_get_user_id
	@echo '--- Zip update-settings-auth function ---'
	zip update-settings-auth.zip ./update_settings
	@echo '--- Zip get-settings-auth function ---'
	zip get-settings-auth.zip ./get_settings
	@echo '--- Zip logout-auth function ---'
	zip logout-auth.zip ./logout
	@echo '--- Zip warm-up-auth function ---'
	zip warmup-auth.zip ./warm_up
	@echo '--- Zip internal-handle-task-auth function ---'
	zip internal_handle_task.zip ./internal_handle_task
	@echo '--- Zip internal-start-auth function ---'
	zip internal_start.zip ./internal_start
	@echo '--- Zip internal-complete-auth function ---'
	zip internal_complete.zip ./internal_complete
	@echo '--- Zip internal-clean-db-auth function ---'
	zip clean.zip ./clean

test-deploy: zip_lambda
	@echo '--- Build lambda test ---'
	@echo 'Package template'
	sam package --template-file auth-template.yaml --s3-bucket ringoid-cloudformation-template --output-template-file auth-template-packaged.yaml
	@echo 'Deploy test-auth-stack'
	sam deploy --template-file auth-template-packaged.yaml --s3-bucket ringoid-cloudformation-template --stack-name test-auth-stack --capabilities CAPABILITY_IAM --parameter-overrides Env=test --no-fail-on-empty-changeset

stage-deploy: zip_lambda
	@echo '--- Build lambda stage ---'
	@echo 'Package template'
	sam package --template-file auth-template.yaml --s3-bucket ringoid-cloudformation-template --output-template-file auth-template-packaged.yaml
	@echo 'Deploy stage-auth-stack'
	sam deploy --template-file auth-template-packaged.yaml --s3-bucket ringoid-cloudformation-template --stack-name stage-auth-stack --capabilities CAPABILITY_IAM --parameter-overrides Env=stage --no-fail-on-empty-changeset

prod-deploy: zip_lambda
	@echo '--- Build lambda prod ---'
	@echo 'Package template'
	sam package --template-file auth-template.yaml --s3-bucket ringoid-cloudformation-template --output-template-file auth-template-packaged.yaml
	@echo 'Deploy prod-auth-stack'
	sam deploy --template-file auth-template-packaged.yaml --s3-bucket ringoid-cloudformation-template --stack-name prod-auth-stack --capabilities CAPABILITY_IAM --parameter-overrides Env=prod --no-fail-on-empty-changeset

clean:
	@echo '--- Delete old artifacts ---'
	rm -rf auth-template-packaged.yaml
	rm -rf start
	rm -rf start-auth.zip
	rm -rf complete
	rm -rf complete-auth.zip
	rm -rf create-auth.zip
	rm -rf create
	rm -rf internal_get_user_id
	rm -rf internal-getuserid-auth.zip
	rm -rf update-settings-auth.zip
	rm -rf update_settings
	rm -rf get-settings-auth.zip
	rm -rf get_settings
	rm -rf logout-auth.zip
	rm -rf logout
	rm -rf warmup-auth.zip
	rm -rf warm_up
	rm -rf internal_handle_upload.zip
	rm -rf internal_handle_upload
	rm -rf internal_handle_task
	rm -rf internal_handle_task.zip
	rm -rf internal_start.zip
	rm -rf internal_start
	rm -rf internal_complete
	rm -rf internal_complete.zip
	rm -rf clean.zip
	rm -rf clean

