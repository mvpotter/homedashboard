# Include variables from the .envrc file
include .envrc
export

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

# Create the new confirm target.
.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## run/homedashboard: run the cmd/homedashboard application
.PHONY: run/homedashboard
run/homedashboard:
	@go run ./cmd/homedashboard

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #
## audit: tidy dependencies and format, vet and test all code
.PHONY: audit
audit:
	@echo 'Tidying and verifying module dependencies...' go mod tidy
	go mod verify
	@echo 'Formatting code...'
	go fmt ./...
	@echo 'Vetting code...'
	go vet ./...
	#staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...

# ==================================================================================== #
# BUILD
# ==================================================================================== #

## build/homedashboard: build the cmd/homedashboard application
.PHONY: build/homedashboard
build/homedashboard:
	@echo 'Building cmd/homedashboard...'
	go build -o=./bin/homedashboard ./cmd/homedashboard
	GOOS=linux GOARCH=amd64 go build -o=./bin/linux_amd64/homedashboard ./cmd/homedashboard
	GOOS=linux GOARCH=arm64 go build -o=./bin/linux_arm64/homedashboard ./cmd/homedashboard

# ==================================================================================== #
# PRODUCTION
# ==================================================================================== #

production_host_ip = '192.168.3.22'

## production/connect: connect to the production server
.PHONY: production/connect
production/connect:
	ssh mpotter@${production_host_ip}

## production/deploy/homedashboard: deploy the api to production
.PHONY: production/deploy/homedashboard
production/deploy/homedashboard:
	rsync -P ./bin/linux_arm64/homedashboard mpotter@${production_host_ip}:~/homedashboard/
	rsync -P ./remote/production/homedashboard.service mpotter@${production_host_ip}:~/homedashboard/
	rsync -aP ./static mpotter@${production_host_ip}:~/homedashboard/
	ssh -t mpotter@${production_host_ip} '\
		sudo mv ~/homedashboard/homedashboard.service /etc/systemd/system/ \
        && sudo systemctl enable homedashboard \
        && sudo systemctl restart homedashboard \
    '
