# Onboard accounts to your Connect platform

Build a Connect integration which creates an account and onboards it to your platform.

Here are some basic scripts you can use to build and run the application.

## Run the sample

1. Install the dependencies

~~
go mod download github.com/stripe/stripe-go/v81
go mod download github.com/gorilla/mux
~~

2. Run the server

~~~
go run server.go
~~~

3. Build the client app

~~~
npm install
~~~

4. Run the client app

~~~
npm start
~~~

5. Go to [http://localhost:4242](http://localhost:4242)