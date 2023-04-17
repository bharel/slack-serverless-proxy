# slack-serverless-proxy

Slack allows up to 2 seconds of response time for any API command sent to a server.
Using a serverless solution with cold instances will sometimes take a large portion of that response time.

This repository is a collection of Go functions, meant to proxy the slack request into a queue for further processing (after verifying the validity / signature), while responding to slack in a timely fashion, even on cold starts.

Go was chosen after careful examination and comparison with other languages. According to different [sources](https://medium.com/google-cloud/serverless-performance-comparison-does-the-language-matter-c72a7191c799), it has the fastest cold start time of all languages common to the serverless infrastructures, has the lowest memory footprint, and offers good performance on hot instances.

After deploying the library, another serverless function can be set to read from the queue, in any chosen programming language, with an unlimited response time.

### Notes

- Does not support directly responding to the slack requests. Automatically returns a 200 on valid requests.
In order to respond, use the data from the queue and send a message in the appropriate channel. For invalid commands, an example solution would be to send an [ephemeral message](https://api.slack.com/methods/chat.postEphemeral).
- Does not support OAuth authentication, as `/auth` requires response to the immediate request. OAuth requests do not have a timeout and can implemented using a different serverless function and endpoint.

## Installation instructions
See the folder applicable to the serverless provider:
- [Google Cloud Functions](/GCF)
