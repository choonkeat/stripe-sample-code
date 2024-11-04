include .env
run:
	go run server.go -secret-key=${STRIPE_SECRET_KEY} -publishable-key=${STRIPE_PUBLISHABLE_KEY} -connected-account-id=${STRIPE_CONNECTED_ACCOUNT_ID}
