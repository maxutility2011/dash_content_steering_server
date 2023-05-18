# mpegdash_content_steering_server_poc

To initialize the repo, 
  - run "go mod init [your repo]".
  - run "go mod tidy".

To run the DASH content steering server, run "go run steering_server.go [default_baseurl]". 

Next, you can start streaming using Dash.js v4.5.1, http://reference.dashif.org/dash.js/v4.5.1/samples/advanced/content-steering.html. Enter the stream url, http://localhost:2210/dash.mpd.
