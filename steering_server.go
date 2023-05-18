package main

import (
    "fmt"
    "os"
    "net/http"
    "encoding/xml"
    "encoding/json"
    "strconv"
    "io/ioutil"
    "strings"
    "github.com/google/uuid"
)

type dashPeriodXml struct {
    XMLName xml.Name `xml:"Period"`
    Value string `xml:",innerxml"`
}

type ContentSteeringXml struct {
    XMLName xml.Name `xml:"ContentSteering"`
    Value string `xml:",innerxml"`
    DefaultServiceLocation string `xml:"defaultServiceLocation,attr,omitempty"` 
    QueryBeforeStart string `xml:"queryBeforeStart,attr,omitempty"` 
    ProxyServerURL string `xml:"proxyServerURL,attr,omitempty"` 
}

type BaseURLXml struct {
    XMLName xml.Name `xml:"BaseURL"`
    Value string `xml:",innerxml"`
    ServiceLocation string `xml:"serviceLocation,attr"`
}

type mpdXml struct {
    XMLName xml.Name `xml:"MPD"`
    MediaPresentationDuration string `xml:"mediaPresentationDuration,attr"`
    MinBufferTime string `xml:"minBufferTime,attr"`
    Profiles string `xml:"profiles,attr"`
    Mpd_type string `xml:"type,attr"`
    Xmlns string `xml:"xmlns,attr,omitempty"`
    Xmlns_xsi string `xml:"xmlns:xsi,attr,omitempty"`
    Xsi_schemaLocation string `xml:"xsi:schemaLocation,attr,omitempty"`
    BaseURLs []BaseURLXml `xml:"BaseURL"`
    ContentSteering ContentSteeringXml `xml:"ContentSteering"`
    Periods []dashPeriodXml `xml:"Period"`
}

// MPEG-DASH Content Steering spec: https://dashif.org/docs/DASH-IF-CTS-00XX-Content-Steering-Community-Review.pdf
// The following definition can be found in Table 6.3.1 in the spec.
type dcsmSpec struct {
    VERSION int `json:"VERSION"`
    TTL int `json:"TTL"`
    RELOAD_URI string `json:"RELOAD-URI"`
    SERVICE_LOCATION_PRIORITY []string `json:"SERVICE-LOCATION-PRIORITY"`
} 

type serviceLocationEntrySpec struct {
    ServiceLocationId string `json:"serviceLocationId"`
    ServiceLocationUri string `json:"serviceLocationUri"`
}

type contentSteeringConfigSpec struct {
    TTL int `json:"TTL"`
    ReloadUri string `json:"RELOAD_URI"`
    ServiceLocations []serviceLocationEntrySpec `json:"serviceLocations"`
}

/*
Sample contentSteeringConfigSpec:

{
    "TTL": 100,
    "RELOAD_URI": "http://localhost:2210/dash.dcsm",
	"serviceLocations": [{
			"serviceLocationId": "baseurl_2",
			"serviceLocationUri": "https://bzhang-zencoder-test.s3.us-west-2.amazonaws.com/bbb2/"
		},
		{
			"serviceLocationId": "baseurl_1",
			"serviceLocationUri": "https://bzhang-zencoder-test.s3.us-west-2.amazonaws.com/bbb/"
		}
	]
}
*/

var dashMpdFileExtension = ".mpd"
var dashContentSteeringManifestFileExtension = ".dcsm"
var content_steering_config_endpoint = "content_steering_config" 

var remoteBaseUrl = "https://bzhang-zencoder-test.s3.us-west-2.amazonaws.com/bbb/"

var server_ip = "localhost"
var server_port = "2210" 
var server_addr = server_ip + ":" + server_port

const session_id_query_param = "sessionId"
const dash_pathway_query_param = "_DASH_pathway"
const dash_throughput_query_param = "_DASH_throughput"

var dcsm_ttl = 10 
const DCSM_VERSION = 1 
var content_steering_server_url = "http://" + server_addr + "/dash.dcsm"
//var serviceLocationMap = map[string]string{"baseurl_1": "https://bzhang-zencoder-test.s3.us-west-2.amazonaws.com/bbb/",
//                                            "baseurl_2": "https://bzhang-zencoder-test.s3.us-west-2.amazonaws.com/bbb2/"} 

var serviceLocationMap []serviceLocationEntrySpec

func main() {
    if len(os.Args) > 1 {
        remoteBaseUrl = os.Args[1]
    }

    fmt.Println("remoteBaseUrl: ", remoteBaseUrl)
     
    http.HandleFunc("/", main_steering_server_handler)

    fmt.Println("Steering server listening on: ", server_addr)
    http.ListenAndServe(server_addr, nil)
    //http.ListenAndServeTLS(server_addr, "steering_server.crt", "steering_server.key", nil)
}

func resolveRemoteUrl(objUrl string) string {
    return (remoteBaseUrl + objUrl)
}

func downloadRemoteMpd(remoteMpdUrl string) []byte {
    resp, err := http.Get(remoteMpdUrl)
    
    if err != nil {
        fmt.Println("Error: Failed to download: ", remoteMpdUrl)
        return nil
    }

    defer resp.Body.Close()

    bodyBytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        fmt.Println("Error: Failed to read response body")
        return nil
    }

    //responseBodyString := string(bodyBytes)
    return bodyBytes
}

func getBaseUrls(base_urls *[]serviceLocationEntrySpec) {
    *base_urls = serviceLocationMap
}

func addContentSteeringInfo(contentSteering *ContentSteeringXml, 
                            steering_server_url string, 
                            defaultServiceLocation string) {
    contentSteering.Value = steering_server_url
    contentSteering.DefaultServiceLocation = defaultServiceLocation
    contentSteering.QueryBeforeStart = "true"
}

func addContentSteeringInfoToMpd(mpdBytes []byte) []byte {
    mpd_xml := mpdXml{}
    err := xml.Unmarshal(mpdBytes, &mpd_xml)
    
    if err != nil {
        fmt.Println("Error: Failed to parse MPD")
        return nil
    }

    //fmt.Println("minBufferTime: ", mpd_xml.MinBufferTime)

    var base_urls []serviceLocationEntrySpec
    getBaseUrls(&base_urls)

    if len(base_urls) == 0 {
        fmt.Println("Error: Failed to generate MPD. Error: no BaseUrl found")
        return nil
    }
    
    // TODO: Need to query cdn switcher for default service location
    default_service_loc := base_urls[0].ServiceLocationId

    for _, sl_entry := range base_urls {
        newBaseUrl := BaseURLXml{ServiceLocation: sl_entry.ServiceLocationId, Value: sl_entry.ServiceLocationUri}
        mpd_xml.BaseURLs = append(mpd_xml.BaseURLs, newBaseUrl)
    }

    // Add ContentSteering element
    addContentSteeringInfo(&mpd_xml.ContentSteering, 
                            content_steering_server_url, 
                            default_service_loc)

    output, err := xml.MarshalIndent(mpd_xml, "  ", "    ")
    if err != nil {
		fmt.Printf("error: %v\n", err)
	}

    fmt.Println("New MPD: .............................\n")
    fmt.Printf("%s\n\n", output)

    return output
}

// TODO: we need to make use of the query params contained in the 
//       DCSM request url (e.g., sessionId, _DASH_pathway and _DASH_throughput) 
//       when determining the service location priority. 
func generateDcsm(sessionId_query_param string, 
                    pathway_query_param string, 
                    throughput_query_param string, 
                    r *http.Request) []byte {
    reloadUri := content_steering_server_url + "?" + session_id_query_param + "=" + sessionId_query_param
    fmt.Println("pathway_query_param: " + pathway_query_param)
    fmt.Println("throughput_query_param: " + throughput_query_param)

    dcsm := dcsmSpec{
        VERSION: DCSM_VERSION,  
        TTL: dcsm_ttl, 
        RELOAD_URI: reloadUri,
        SERVICE_LOCATION_PRIORITY: []string{},
    }

    var base_urls []serviceLocationEntrySpec
    getBaseUrls(&base_urls)

    if len(base_urls) == 0 {
        return nil
    }

    // TODO: Need to query cdn switcher for the service location priority
    for _, sl_entry := range base_urls {
        dcsm.SERVICE_LOCATION_PRIORITY = append(dcsm.SERVICE_LOCATION_PRIORITY, sl_entry.ServiceLocationId)
    }

    dcsmBytes, err := json.Marshal(dcsm)
    if err != nil {
		fmt.Println("Failed to marshal DCSM JSON object:", err)
	}

    fmt.Println("DCSM response: .............................\n")
    fmt.Printf("%s\n\n", dcsmBytes)
    return dcsmBytes
}

func updateDefaultServiceLocationPriority(slm contentSteeringConfigSpec) string {
    serviceLocationMap_old := serviceLocationMap
    serviceLocationMap = nil
     
    for _, sl := range slm.ServiceLocations {
        if sl.ServiceLocationId == "" {
            fmt.Println("Error: Empty ServiceLocationId")
            serviceLocationMap = serviceLocationMap_old
            return "Empty ServiceLocation"
        }

        if sl.ServiceLocationUri == "" {
            fmt.Println("Error: Empty ServiceLocationUri")
            serviceLocationMap = serviceLocationMap_old
            return "Empty ServiceLocationUri"
        }

        new_sl := sl
        serviceLocationMap = append(serviceLocationMap, new_sl)
    }

    serviceLocationMap_old = nil
    return ""
}

func updateDefaultContentSteeringConfig(csc contentSteeringConfigSpec) string {
    if len(csc.ReloadUri) == 0 {
        return "Empty RELOAD-URI config"
    }

    if csc.TTL <= 0 {
        return "Non-positive TTL value"
    }

    content_steering_server_url = csc.ReloadUri
    dcsm_ttl = csc.TTL

    return updateDefaultServiceLocationPriority(csc)
}

func respondMpd(responseBytes []byte, w http.ResponseWriter) int {
    enableCors(w)

    FileContentType := "application/dash+xml"
    w.Header().Set("Content-Type", FileContentType)
    w.Header().Set("Content-Length", strconv.FormatInt(int64(len(responseBytes)), 10))

    w.Write(responseBytes)
    return 0
}

func respondDcsm(responseBytes []byte, w http.ResponseWriter) int {
    enableCors(w)

    FileContentType := "application/json"
    w.Header().Set("Content-Type", FileContentType)
    w.Header().Set("Content-Length", strconv.FormatInt(int64(len(responseBytes)), 10))

    w.Write(responseBytes)
    return 0
}

func enableCors(w http.ResponseWriter) {
    w.Header().Set("Access-Control-Allow-Origin", "*")
}

func main_steering_server_handler(w http.ResponseWriter, r *http.Request) {
    fmt.Println("----------------------------------------")
    fmt.Println("Received new request:")
    fmt.Println(r.Method, r.URL.Path)

    posLastSingleSlash := strings.LastIndex(r.URL.Path, "/")
    UrlLastPart := r.URL.Path[posLastSingleSlash + 1 :]

    // Remove trailing "/" if any
    if len(UrlLastPart) == 0 {
        path_without_trailing_slash := r.URL.Path[0 : posLastSingleSlash]
        posLastSingleSlash = strings.LastIndex(path_without_trailing_slash, "/")
        UrlLastPart = path_without_trailing_slash[posLastSingleSlash + 1 :]
    } 

    if !(UrlLastPart == content_steering_config_endpoint || strings.Contains(UrlLastPart, dashContentSteeringManifestFileExtension) || strings.Contains(UrlLastPart, dashMpdFileExtension)) {
        err := "Endpoint = " + UrlLastPart + " is not supported."
        fmt.Println(err)
        http.Error(w, "403 forbidden\n  Error: " + err, http.StatusForbidden)
        return
    }

    // Handle content steering configuration requests
    if UrlLastPart == content_steering_config_endpoint {
        // TODO: We should support "GET" content steering config. 
        //       Currently, we only handle "POST" requests to this endpoint
        if r.Method != "POST" {
            err := "Method = " + r.Method + " is not allowed to " + r.URL.Path
            fmt.Println(err)
            http.Error(w, "405 method not allowed\n  Error: " + err, http.StatusMethodNotAllowed)
            return
        }

        var csc contentSteeringConfigSpec
        err := json.NewDecoder(r.Body).Decode(&csc)
        if err != nil {
            res := "Failed to decode the received content steering configuration. Is it valid JSON?"
            fmt.Println("Error happened in JSON marshal. Err: %s", err)
            http.Error(w, "400 bad request\n  Error: " + res, http.StatusBadRequest)
            return
        }

        res := updateDefaultContentSteeringConfig(csc)
        if res == "" {
            w.WriteHeader(http.StatusAccepted)
            w.Header().Set("Content-Type", "application/json")

            jsonResp, err := json.MarshalIndent(csc, " ", "  ")
	        if err != nil {
                fmt.Println("Error happened in JSON marshal. Err: %s", err)
                http.Error(w, "500 Internal server error", http.StatusInternalServerError)
                return
	        }

	        w.Write(jsonResp)
            return
        } else if res != "" {
            fmt.Println("Error: ", res)
            http.Error(w, "400 bad request.\n  Error: " + res, http.StatusBadRequest)
            return
        }

        return
    }

    posLastDotIn_UrlLastPart := strings.LastIndex(UrlLastPart, ".")
    UrlLastPart_fileExtension := UrlLastPart[posLastDotIn_UrlLastPart :]

    if strings.Contains(UrlLastPart_fileExtension, dashMpdFileExtension) {
        if r.Method == "OPTIONS" {
            w.Header().Add("Access-Control-Allow-Methods", "GET") 
            w.WriteHeader(http.StatusOK)
            return
        } else if r.Method == "GET" {
            remoteMpdUrl := resolveRemoteUrl(UrlLastPart)
            mpdBytes := downloadRemoteMpd(remoteMpdUrl)

            // Add the BaseUrl elements and the ContentSteering element to MPD
            newMpdBytes := addContentSteeringInfoToMpd(mpdBytes)
            if newMpdBytes != nil {
                respondMpd(newMpdBytes, w)
            } else {
                fmt.Println("Error: Failed to generate MPD")
                http.Error(w, "500 Internal server error\n  Error: Failed to generate MPD\nHave you created content steering config first?\ne.g. POST /content_steering_config", http.StatusInternalServerError)
                return
            }

            return
        } else {
            err := "Error: method = " + r.Method + " is not allowed to " + r.URL.Path
            fmt.Println()
            http.Error(w, "405 method not allowed.\n  Error: " + err, http.StatusMethodNotAllowed)
            return
        }
    } else if strings.Contains(UrlLastPart_fileExtension, dashContentSteeringManifestFileExtension) {
        var sid string
        var pathway string
        var throughput string

        // According to the spec, https://dashif.org/docs/DASH-IF-CTS-00XX-Content-Steering-Community-Review.pdf
        // the very first DCSM request in a session does not contain query params
        if len(r.URL.Query()) == 0 {
            sid = uuid.New().String()
            fmt.Println("Start of a new session. Generating a new session ID: ", sid)
        } else {
            sid = r.URL.Query().Get(session_id_query_param)
            pathway = r.URL.Query().Get(dash_pathway_query_param)
            throughput = r.URL.Query().Get(dash_throughput_query_param)
        }

        if r.Method == "OPTIONS" {
            w.Header().Add("Access-Control-Allow-Methods", "GET") 
            w.WriteHeader(http.StatusOK)
            return
        } else if r.Method == "GET" {
            // Generate DASH Content Steering Manifest (DCSM) for the requesting client
            dcsmBytes := generateDcsm(sid, pathway, throughput, r) 
            if dcsmBytes == nil {
                err := "No DCSM configuration found. Please create DCSM configuration first"
                fmt.Println(err)
                http.Error(w, "404 not found\n  Error: " + err, http.StatusBadRequest)
                return
            }

            respondDcsm(dcsmBytes, w)
            return
        } else {
            err := "Error: method = " + r.Method + " is not allowed to " + r.URL.Path
            fmt.Println(err)
            http.Error(w, "405 method not allowed.\n  Error: " + err, http.StatusMethodNotAllowed)
            return
        }
    } 

    http.Error(w, "400 bad request", http.StatusBadRequest)
    return
}