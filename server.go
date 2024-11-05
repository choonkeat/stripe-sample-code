package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/account"
	"github.com/stripe/stripe-go/v81/accountlink"
	"github.com/stripe/stripe-go/v81/paymentintent"
	"github.com/stripe/stripe-go/v81/transfer"
)

type CheckoutData struct {
	PublishableKey     string
	ClientSecret       string
	ConnectedAccountID string
	ReturnURL          string
}

type CompleteData struct {
	PublishableKey     string
	ConnectedAccountID string
}

// Flags struct to hold configuration values
type Flags struct {
	SecretKey          string
	PublishableKey     string
	ConnectedAccountID string
}

func main() {
	// Define command-line flags
	var flags Flags
	flag.StringVar(&flags.SecretKey, "secret-key", os.Getenv("STRIPE_SECRET_KEY"), "Your Stripe secret API key")
	flag.StringVar(&flags.PublishableKey, "publishable-key", os.Getenv("STRIPE_PUBLISHABLE_KEY"), "Your Stripe publishable API key")
	flag.StringVar(&flags.ConnectedAccountID, "connected-account-id", os.Getenv("STRIPE_CONNECTED_ACCOUNT_ID"), "Your Stripe connected account ID")

	// Parse the flags
	flag.Parse()

	// Ensure that required flags are provided
	if flags.SecretKey == "" || flags.PublishableKey == "" || flags.ConnectedAccountID == "" {
		log.Fatal("Please provide -secret-key, -publishable-key, and -connected-account-id flags")
	}

	// Set your secret key. Remember to switch to your live secret key in production.
	// See your keys here: https://dashboard.stripe.com/apikeys
	stripe.Key = flags.SecretKey

	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	// Stripe Connect onboarding
	// https://docs.stripe.com/connect/onboarding/quickstart#init-stripe
	r.HandleFunc("/account", CreateAccount)
	r.HandleFunc("/account_link", CreateAccountLink)

	// PaymentIntent
	// https://docs.stripe.com/connect/direct-charges?platform=web&ui=elements
	checkoutTmpl := template.Must(template.ParseFiles("views/checkout.html"))
	completeTmpl := template.Must(template.ParseFiles("views/checkout/complete.html"))
	r.HandleFunc("/checkout", func(w http.ResponseWriter, r *http.Request) {
		params := &stripe.PaymentIntentParams{
			Amount:   stripe.Int64(1000),
			Currency: stripe.String(string(stripe.CurrencyUSD)),
			AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
				Enabled: stripe.Bool(true),
			},
			ApplicationFeeAmount: stripe.Int64(123),
		}

		connectedAccountID := flags.ConnectedAccountID
		params.SetStripeAccount(connectedAccountID)
		intent, err := paymentintent.New(params)
		if err != nil {
			log.Printf("An error occurred when calling the Stripe API to create a payment intent: %v", err)
			handleError(w, err)
			return
		}

		data := CheckoutData{
			PublishableKey:     flags.PublishableKey,
			ClientSecret:       intent.ClientSecret,
			ConnectedAccountID: connectedAccountID,
			ReturnURL:          fmt.Sprintf("http://localhost:4242/checkout/complete?account=%s", connectedAccountID),
		}
		checkoutTmpl.Execute(w, data)
	})
	r.HandleFunc("/checkout/complete", func(w http.ResponseWriter, r *http.Request) {
		// https://docs.stripe.com/connect/direct-charges?platform=web&ui=elements#handle-post-payment-events
		accountID := r.URL.Query().Get("account")
		data := CompleteData{
			PublishableKey:     flags.PublishableKey,
			ConnectedAccountID: accountID,
		}
		completeTmpl.Execute(w, data)
	})

	// Transfers API
	// A Transfer object is created when moving funds between Stripe accounts on a Connect platform.
	// https://docs.stripe.com/api/transfers/create
	r.HandleFunc("/transfers/create", func(w http.ResponseWriter, r *http.Request) {
		stripe.Key = flags.SecretKey

		params := &stripe.TransferParams{
			Amount:      stripe.Int64(1),
			Currency:    stripe.String(string(stripe.CurrencySGD)),
			Destination: stripe.String(flags.ConnectedAccountID),
			SourceType:  stripe.String(string(stripe.TransferSourceTypeCard)),
		}
		result, err := transfer.New(params)
		if err != nil {
			// {"error":"You have insufficient available funds in your Stripe account. Try adding funds directly to your available balance by creating Charges using the 4000000000000077 test card. See: https://stripe.com/docs/testing#available-balance"}
			log.Printf("An error occurred when calling the Stripe API to create a transfer: %v", err)
			handleError(w, err)
			return
		}
		writeJSON(w, result)
	})

	r.PathPrefix("/").HandlerFunc(CatchAllHandler) // SPA stuff
	http.Handle("/", r)
	addr := "localhost:4242"
	log.Printf("Listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func CatchAllHandler(w http.ResponseWriter, r *http.Request) {
	filePath := "dist/" + r.URL.Path

	if _, err := os.Stat(filepath.Clean(filePath)); os.IsNotExist(err) {
		// if the requested file doesn't exist, serve index.html
		http.ServeFile(w, r, "dist/index.html")
		return
	}

	http.ServeFile(w, r, filePath)
}

type RequestBody struct {
	Account string `json:"account"`
}

func CreateAccountLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	var requestBody RequestBody
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	accountLink, err := accountlink.New(&stripe.AccountLinkParams{
		Account:    stripe.String(requestBody.Account),
		ReturnURL:  stripe.String(fmt.Sprintf("http://localhost:4242/return/%s", requestBody.Account)),
		RefreshURL: stripe.String(fmt.Sprintf("http://localhost:4242/refresh/%s", requestBody.Account)),
		Type:       stripe.String("account_onboarding"),
	})

	if err != nil {
		log.Printf("An error occurred when calling the Stripe API to create an account link: %v", err)
		handleError(w, err)
		return
	}
	writeJSON(w, struct {
		URL string `json:"url"`
	}{
		URL: accountLink.URL,
	})
}

func CreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	account, err := account.New(&stripe.AccountParams{})

	if err != nil {
		log.Printf("An error occurred when calling the Stripe API to create an account: %v", err)
		handleError(w, err)
		return
	}

	writeJSON(w, struct {
		Account string `json:"account"`
	}{
		Account: account.ID,
	})
}

func handleError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	if stripeErr, ok := err.(*stripe.Error); ok {
		writeJSON(w, struct {
			Error string `json:"error"`
		}{
			Error: stripeErr.Msg,
		})
	} else {
		writeJSON(w, struct {
			Error string `json:"error"`
		}{
			Error: err.Error(),
		})
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("json.NewEncoder.Encode: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, &buf); err != nil {
		log.Printf("io.Copy: %v", err)
		return
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a wrapper for the ResponseWriter to capture the status code
		wrapper := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(wrapper, r)

		// Log the response status
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, wrapper.status, http.StatusText(wrapper.status))
	})
}

// responseWriter is a wrapper for http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code before writing it
func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
