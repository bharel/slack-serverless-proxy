# Slack Google Cloud Functions Proxy
Slack function proxy built for Google Cloud Functions.

## Installation
Deploy `/src` to Google Cloud Functions.

Supply the following environment variables:

- `SLACK_SIGNING_SECRET`: Signing secret when creating a slack bot.
- `GCP_PROJECT`: Google Cloud Project id.
- `PUBSUB_TOPIC`: Pub/Sub topic id, to send the slack messages to.

The messages will be sent to the topic unmodified after verifying the signature.
