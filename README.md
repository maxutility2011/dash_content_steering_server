# mpegdash_content_steering_server_poc

To initialize the repo, 
  - run "go mod init bithub.brightcove.com/Research/mpegdash_content_steering_server_poc".
  - run "go mod tidy".

Before running steering_server.go, please read the PoC design doc here, https://docs.google.com/presentation/d/1l3HBGfflMvaDoQP1OZqsNypkVMF3LswJ/. A POSTMAN collection for all the request supported by the server is available to download at https://drive.google.com/file/d/1GCRxeG_RwQonYLZOTYG026YIP_dq-Nax/.

To run the DASH content steering server, run "go run steering_server.go [default_baseurl]". [default_baseurl] is the default service location (cdn) from where steering_server.go can download the dash stream. For example, my demo hosts two copies of the same dash stream on two different S3 folders,
https://bzhang-zencoder-test.s3.us-west-2.amazonaws.com/freedom_1/ and https://bzhang-zencoder-test.s3.us-west-2.amazonaws.com/freedom_2/. The default folder is the first. So you start the server by running "go run steering_server.go https://bzhang-zencoder-test.s3.us-west-2.amazonaws.com/freedom_1/". The server listens on port http://localhost:2210. 

After the server runs, the first thing to do is to configure content steering by "POST"(ing) to the "/content_steering_config" endpoint with a JSON configuration. Please refer to the POSTMAN collection - request "update service location priority". This request will set the initial content steering configs, such as the service location (aka. BaseURLs) priority, TTL of the content steering manifest, the content steering manifest's reload-uri.

Next, you can start streaming using Dash.js v4.5.1, http://reference.dashif.org/dash.js/v4.5.1/samples/advanced/content-steering.html. Enter the stream url, http://localhost:2210/dash.mpd.
