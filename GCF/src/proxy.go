package proxy

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"unsafe"

	"cloud.google.com/go/pubsub"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

var (
	slackSigningSecret []byte
	pubsubClient       *pubsub.Client
	topic              *pubsub.Topic
)

const maxBodySize = 1024 * 1024 * 10 // 10MB

func init() {
	// Get the Slack signing secret from the environment
	slackSigningSecret = []byte(os.Getenv("SLACK_SIGNING_SECRET"))
	if len(slackSigningSecret) == 0 {
		log.Panicln("SLACK_SIGNING_SECRET env var must be set.")
	}

	// Get the GCP project from the environment
	project := os.Getenv("GCP_PROJECT")
	if project == "" {
		log.Panicln("GCP_PROJECT env var must be set.")
	}

	// Get the Pub/Sub topic ID from the environment
	topicName := os.Getenv("PUBSUB_TOPIC")
	if topicName == "" {
		log.Panicln("PUBSUB_TOPIC env var must be set.")
	}

	var err error

	// Create a Pub/Sub client
	pubsubClient, err = pubsub.NewClient(context.Background(), project)
	if err != nil {
		log.Panicf("Failed creating a Pub/Sub client: %s.", err.Error())
	}

	// Get the topic
	topic = pubsubClient.Topic(topicName)

	if exists, err := topic.Exists(context.Background()); err != nil || !exists {
		log.Panicf("Topic %s doesn't exist.\n", topicName)
	}

	topic.PublishSettings.CountThreshold = 1

	// Register the function
	functions.HTTP("Proxy", Proxy)
}

// stringToByteSlice converts a string to a byte slice without copying the underlying data.
func stringToByteSlice(s *string) []byte {
	return unsafe.Slice(unsafe.StringData(*s), len(*s))
}

// byteSliceToString converts a byte slice to a string without copying the underlying data.
func byteSliceToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// Validate the Slack signature
// Returns true if valid, false otherwise
// https://api.slack.com/authentication/verifying-requests-from-slack
// Reads the body but restores it before returning
func isValidSlackSignature(secret []byte, r *http.Request) bool {
	// Get the timestamp from the request header
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")

	// Get the signature from the request header
	signature := r.Header.Get("X-Slack-Signature")

	// Read the body
	body := make([]byte, r.ContentLength)
	if _, err := io.ReadFull(r.Body, body); err != nil {
		return false
	}

	// Close the body before replacing it
	r.Body.Close()

	// Reset the body so it can be read again
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Create the expected signature
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, byteSliceToString(body))
	signatureHash := hmac.New(sha256.New, secret)
	signatureHash.Write(stringToByteSlice(&baseString))
	expectedSignature := fmt.Sprintf("v0=%s", hex.EncodeToString(signatureHash.Sum(nil)))

	// Compare the signatures
	if !hmac.Equal(stringToByteSlice(&signature), stringToByteSlice(&expectedSignature)) {
		return false
	}

	return true
}

// Validate a request
// Returns 0 if valid, HTTP status code otherwise
func validateRequest(r *http.Request) int {
	if r.Method != http.MethodPost {
		return http.StatusMethodNotAllowed
	}

	if r.Header.Get("Content-Type") != "application/json" {
		return http.StatusUnsupportedMediaType
	}

	if r.ContentLength > maxBodySize {
		return http.StatusRequestEntityTooLarge
	}

	if r.ContentLength <= 0 {
		return http.StatusBadRequest
	}

	if r.Body == nil {
		return http.StatusBadRequest
	}

	if !isValidSlackSignature(slackSigningSecret, r) {
		return http.StatusUnauthorized
	}

	return 0
}

// Proxy a slack request to Pub/Sub
// Makes sure the request is a valid slack request before proxying it
func Proxy(w http.ResponseWriter, r *http.Request) {

	// Validate the request
	if status := validateRequest(r); status != 0 {
		w.WriteHeader(status)
		log.Printf("Invalid request. Returned status: %d", status)
		return
	}

	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// Technically this should never happen
		// (already read the body on validateRequest)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	msg := pubsub.Message{
		Data: body,
	}

	// Publish the message
	publishResult := topic.Publish(r.Context(), &msg)

	// Ensure the message was published
	if _, err := publishResult.Get(r.Context()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("Failed publishing message: ", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}
