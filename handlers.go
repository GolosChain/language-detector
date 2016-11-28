package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	rj "github.com/bottlenose-inc/rapidjson" // faster json handling
)

// SendErrorResponse sends a response with the provided error message and status code.
func SendErrorResponse(w http.ResponseWriter, message string, status int) {
	errorsCounter.Inc()
	errorJson := rj.NewDoc()
	defer errorJson.Free()
	errorCt := errorJson.GetContainerNewObj()
	errorCt.AddValue("error", message)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(errorJson.Bytes())
	if err != nil {
		logger.Error("Error writing response: "+err.Error(), map[string]string{"error": errorJson.String()})
	}
}

// GetRequests is a generic function that parses properly formatted requests to an augmentation.
// It ensures the correct Content-Type header is provided and ensures the request is properly
// formatted. Requests are also truncated to BODY_LIMIT_BYTES to avoid huge requests causing problems.
func GetRequests(w http.ResponseWriter, r *http.Request) (*rj.Doc, error) {
	var emptyMap *rj.Doc

	// Send error response if incorrect Content-Type is provided
	if r.Header.Get("Content-Type") != "application/json" {
		invalidRequestsCounter.Inc()
		logger.Warning("Client request did not set Content-Type header to application/json", map[string]string{"Content-Type": r.Header.Get("Content-Type")})
		SendErrorResponse(w, "Content-Type must be set to application/json", http.StatusBadRequest)
		return emptyMap, errors.New("Content-Type must be set to application/json")
	}

	// Read body up to size of BODY_LIMIT_BYTES
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, BODY_LIMIT_BYTES))
	if err != nil {
		logger.Error("Error reading request body: " + err.Error())
		SendErrorResponse(w, "Error reading request body", http.StatusInternalServerError)
		return emptyMap, err
	}
	if err := r.Body.Close(); err != nil {
		logger.Error("Error closing body: " + err.Error())
		SendErrorResponse(w, "Error reading request body", http.StatusInternalServerError)
		return emptyMap, err
	}

	// Parse request JSON
	requestJson, err := rj.NewParsedJson(body)
	if err != nil {
		invalidRequestsCounter.Inc()
		logger.Warning("Client request was invalid JSON: "+err.Error(), map[string]string{"body": string(body)})
		SendErrorResponse(w, "Unable to parse request - invalid JSON detected", http.StatusBadRequest)
		requestJson.Free()
		return emptyMap, err
	}

	return requestJson, err
}

// HandlerWrapper is "wrapped" around all handlers to allow generation of
// common metrics we want for every valid api call.
func HandlerWrapper(handler http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		http.HandlerFunc(handler).ServeHTTP(w, r)
		totalRequestsCounter.Inc()
		requestDurationCounter.Add(time.Since(start).Seconds() / 1000)
	})
}

// NotFound sends a 404 response.
func NotFound(w http.ResponseWriter, r *http.Request) {
	invalidRequestsCounter.Inc()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, err := w.Write(notFound)
	if err != nil {
		// Should not run into this error...
		logger.Error("Error encoding 404 response: " + err.Error())
	}
}

// Usage sends the usage information response.
func Usage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(usage)
	if err != nil {
		// Should not run into this error...
		logger.Error("Error encoding usage response: " + err.Error())
	}
}

// detect language
func LanguageDetectorHandler(w http.ResponseWriter, r *http.Request) {
	requestJson, err := GetRequests(w, r)
	if err != nil {
		incUnsuccessfulCounter()
		return
	}
	defer requestJson.Free()
	requestCt := requestJson.GetContainer()
	if requestCt.GetType() == rj.TypeNull {
		return
	}
	requestsCt, err := requestCt.GetMember("request")
	if err != nil {
		invalidRequestsCounter.Inc()
		logger.Warning("Client request was invalid JSON: " + err.Error())
		SendErrorResponse(w, "Unable to parse request - invalid JSON detected", http.StatusBadRequest)
		return
	}
	requests, _, err := requestsCt.GetArray()

	respCode := http.StatusOK
	responses := rj.NewDoc()
	defer responses.Free()
	responsesCt := responses.GetContainerNewObj()
	responsesArray := responses.NewContainerArray()
	responsesCt.AddMember("response", responsesArray)
	responsesArray, _ = responsesCt.GetMember("response")
	for _, request := range requests {
		response := responses.NewContainerObj()
		text, err := request.GetMember("text")

		if err != nil {
			incUnsuccessfulCounter()
			response.AddValue("error", "Missing text key")
			respCode = http.StatusBadRequest
			err = responsesArray.ArrayAppendContainer(response)
			if err != nil {
				SendErrorResponse(w, err.Error(), http.StatusInternalServerError)
				return
			}
			continue
		}

		textStr, err := text.GetString()
		textStr = StripExtras(textStr)

		code := Detect_language(textStr)
		name, found := KnownLanguages[code]

		if !found {
			name = "Unknown"
			respCode = http.StatusNonAuthoritativeInfo
			logger.Warning("Unknown response language code: " + code)
		}

		response.AddValue("iso6391code", code)
		response.AddValue("name", name)

		incLanguageCount(name)

		// Append newly generated response to responses
		err = responsesArray.ArrayAppendContainer(response)
		if err != nil {
			incUnsuccessfulCounter()
			SendErrorResponse(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Call logProcessed for every object that gets processed
		incSuccessfulCounter()
		logProcessed()
	}

	// Send response
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(respCode)
	_, err = w.Write(responses.Bytes())
	if err != nil {
		// Should not run into this error...
		logger.Error("Error encoding error response: "+err.Error(), map[string]string{"response": responses.String()})
	}
}

func HasPrefix(word string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(word, prefix) {
			return true
		}
	}
	return false
}

// remove mentions and links from text, as these can skew detection
func StripExtras(text string) string {
	var result string

	prefixes := []string{"@", "http"}

	for _, word := range strings.Fields(text) {
		if !HasPrefix(word, prefixes) {
			result = result + word + " "
		}
	}

	return result
}
