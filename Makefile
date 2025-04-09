# include .env
# export

# PSQLUSER := $(shell yq e '.postgresql.primary_db.db_username' config/config.yml)
# PSQLPASS := $(shell yq e '.postgresql.primary_db.db_password' config/config.yml)
# PSQLHOST := $(shell yq e '.postgresql.primary_db.db_host' config/config.yml)
# PSQLPORT := $(shell yq e '.postgresql.primary_db.db_port' config/config.yml)
# PSQLDB := $(shell yq e '.postgresql.primary_db.db_name' config/config.yml)

proto:
	protoc microservices/the_monkeys_gateway/internal/**/pb/*.proto --go_out=. --go-grpc_out=.
	protoc microservices/the_monkeys_authz/internal/pb/*.proto --go_out=. --go-grpc_out=.
	protoc microservices/the_monkeys_users/internal/pb/*.proto --go_out=. --go-grpc_out=.
	protoc microservices/the_monkeys_blog/internal/pb/*.proto --go_out=. --go-grpc_out=.
	protoc microservices/the_monkeys_storage/internal/pb/*.proto --go_out=. --go-grpc_out=.

proto-gen-interservices:
	protoc apis/interservice/**/*.proto --go_out=. --go-grpc_out=.

sql-gen:
	echo "Enter the file's name or description (Node keep it short):"
	@read INPUT_VALUE; \
	migrate create -ext sql -dir schema -seq $$INPUT_VALUE

migrate-up:
	migrate -path schema -database "postgresql://${PSQLUSER}:${PSQLPASS}@${PSQLHOST}:${PSQLPORT}/${PSQLDB}?sslmode=disable" -verbose up

migrate-down:
	migrate -path schema -database "postgresql://${PSQLUSER}:${PSQLPASS}@${PSQLHOST}:${PSQLPORT}/${PSQLDB}?sslmode=disable" -verbose down 1

migrate-force:
	echo "Enter a version:"
	@read INPUT_VALUE; \
	migrate -path schema -database "postgresql://${PSQLUSER}:${PSQLPASS}@${PSQLHOST}:${PSQLPORT}/${PSQLDB}?sslmode=disable" -verbose force $$INPUT_VALUE

proto-gen:
	protoc apis/serviceconn/**/pb/*.proto --go_out=. --go-grpc_out=.
        # --exclude=apis/serviceconn/gateway_recom/pb/other_file.proto  # Include other files explicitly
    # To Generate Python code
	python3 -m grpc_tools.protoc \
        -I=apis/serviceconn \
        -I=apis/python/recom_engine \
        apis/serviceconn/gateway_recom/pb/*.proto \
        --python_out=apis/python/recom_engine \
        --grpc_python_out=apis/python/recom_engine
