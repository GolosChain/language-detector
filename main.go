package main

// #cgo CFLAGS: -I. -fpic
// #cgo LDFLAGS: -L. -lcld2 wrapper.a
// #include <stdlib.h>
// #include "wrapper.h"
import "C"

import (
	"unsafe"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	bnLogger "github.com/bottlenose-inc/go-common-tools/logger" // go-common-tools bunyan-style logger package
	"github.com/bottlenose-inc/go-common-tools/metrics"         // go-common-tools Prometheus metrics package
	rj "github.com/bottlenose-inc/rapidjson"                    // faster json handling
	"github.com/gorilla/mux"                                    // URL router and dispatcher
	"github.com/prometheus/client_golang/prometheus"            // Prometheus client library
)

const (
	AUGMENTATION_NAME = "language_detector"
	PROMETHEUS_NAME   = "language_detector"
	BODY_LIMIT_BYTES  = 1048576 // Truncates incoming requests to 1 mb
	OBJECTS_PER_LOG   = 1000    // Number of objects processed per throughput log message

	USAGE_STRING = `{
  "result": {
    "id": "language-detector",
    "name": "language-detector",
    "description": "Determine language code from text",
    "in": {
      "text": {
        "type": "string"
      }
    },
    "out": {
      "iso6391code": {
        "type": "string"
      },
      "name" : {
        "type" : "string"
      }
    }
  }
}`

	LANG_FILE = "data/cld_codes.json"
)

var (
	LISTEN_PORT     = 3000  // Can be overwritten with the LISTEN_PORT env var
	PROMETHEUS_PORT = 30000 // Can be overwritten with the PROMETHEUS_PORT env var

	numProcessed               = 0
	startTime                  = time.Now()
	totalRequestsCounter       prometheus.Counter
	invalidRequestsCounter     prometheus.Counter
	objsProcessedCounterVector *prometheus.CounterVec
	resultLangCounterVector    *prometheus.CounterVec
	requestDurationCounter     prometheus.Counter
	errorsCounter              prometheus.Counter

	notFound       []byte
	usage          []byte
	logger         *bnLogger.Logger
	KnownLanguages = make(map[string]string)
)

func Detect_language(text string) string {
	cStr := C.CString(text)
	defer C.free(unsafe.Pointer(cStr))
	return C.GoString(C.detect_language(cStr))
}

func main() {
	// Initialize logger
	var err error
	logger, err = bnLogger.NewLogger(AUGMENTATION_NAME)
	if err != nil {
		log.Fatal("Unable to initialize bn logger, exiting: " + err.Error())
	}

	// Start Prometheus metrics server
	if os.Getenv("PROMETHEUS_PORT") != "" {
		if port, err := strconv.Atoi(os.Getenv("PROMETHEUS_PORT")); err != nil {
			logger.Warning("Invalid Prometheus port provided, continuing with default", map[string]string{"provided": os.Getenv("PROMETHEUS_PORT")}, map[string]string{"default": strconv.Itoa(PROMETHEUS_PORT)})
		} else if port > 0 {
			PROMETHEUS_PORT = port
		}
	}
	go metrics.StartPrometheusMetricsServer(AUGMENTATION_NAME, logger, PROMETHEUS_PORT)

	// Initialize Prometheus Metrics
	InitMetrics()

	// Prepare responses
	GenerateResponses()

	// Set listen port based on env, if provided
	if os.Getenv("LISTEN_PORT") != "" {
		if port, err := strconv.Atoi(os.Getenv("LISTEN_PORT")); err != nil {
			logger.Warning("Invalid listen port provided, continuing with default", map[string]string{"provided": os.Getenv("LISTEN_PORT")}, map[string]string{"default": strconv.Itoa(LISTEN_PORT)})
		} else if port > 0 {
			LISTEN_PORT = port
		}
	}

	// load known languages/codes
	langFile, err := ioutil.ReadFile(LANG_FILE)
	if err != nil {
		logger.Fatal("File error: " + err.Error())
		os.Exit(1)
	}
	err = json.Unmarshal(langFile, &KnownLanguages)
	if err != nil {
		logger.Fatal("Error loading known languages: " + err.Error())
		os.Exit(1)
	}

	// Start HTTP server
	err = http.ListenAndServe(":"+strconv.Itoa(LISTEN_PORT), getRouter())
	if err != nil {
		logger.Fatal("Error starting HTTP server: " + err.Error())
		os.Exit(1)
	}
}

// Initialize prometheus metrics
func InitMetrics() {
	var emptyMap map[string]string
	totalRequestsCounter, _ = metrics.CreateCounter("augmentation_requests_total", "", "", "The total number of requests received.", emptyMap)
	invalidRequestsCounter, _ = metrics.CreateCounter("augmentation_invalid_requests_total", "", "", "The total number of invalid requests received.", emptyMap)
	requestDurationCounter, _ = metrics.CreateCounter("augmentation_request_duration_milliseconds", "", "", "The total amount of time spent processing requests.", emptyMap)
	errorsCounter, _ = metrics.CreateCounter("augmentation_errors_logged_total", "", "", "The total number of errors logged.", emptyMap)
	objsProcessedCounterVector, _ = metrics.CreateCounterVector("augmentation_objects_processed_total", "", "", "The total number of objects processed.", emptyMap, []string{"status"})
	metrics.InitCounterVector(objsProcessedCounterVector, []string{"successful", "unsuccessful"})
	resultLangCounterVector, _ = metrics.CreateCounterVector("augmentation_detected_language", "", "", "Counts of languages detected.", emptyMap, []string{"language"})
}

// GenerateResponses prepares the usage and 404 responses. They can then just be returned,
// rather than generated for each individual request.
func GenerateResponses() {
	// Generate usage response
	usageJson, err := rj.NewParsedStringJson(USAGE_STRING)
	if err != nil {
		logger.Fatal("Error generating usage JSON: " + err.Error())
		os.Exit(1)
	}
	usage = usageJson.Bytes()
	usageJson.Free()

	// Generate 404 response
	notFoundJson := rj.NewDoc()
	notFoundCt := notFoundJson.GetContainerNewObj()
	notFoundCt.AddValue("error", "Not found")
	notFound = notFoundJson.Bytes()
	notFoundJson.Free()
}

// Initialize router and define routes
func getRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	router.NotFoundHandler = HandlerWrapper(NotFound)
	router.Methods("GET").Path("/").Handler(HandlerWrapper(Usage))
	router.Methods("POST").Path("/").Handler(HandlerWrapper(LanguageDetectorHandler))
	return router
}

// incSuccessfulCounter increments objsProcessedCounterVector's successful count.
func incSuccessfulCounter() {
	counter, err := objsProcessedCounterVector.GetMetricWithLabelValues("successful")
	if err != nil {
		logger.Error("Incrementing successful objects prometheus counter vector failed: " + err.Error())
	} else {
		counter.Inc()
	}
}

// incSuccessfulCounter increments objsProcessedCounterVector's unsuccessful count.
func incUnsuccessfulCounter() {
	counter, err := objsProcessedCounterVector.GetMetricWithLabelValues("unsuccessful")
	if err != nil {
		logger.Error("Incrementing unsuccessful objects prometheus counter vector failed: " + err.Error())
	} else {
		counter.Inc()
	}
}

// increase language count
func incLanguageCount(language string) {
	counter, err := resultLangCounterVector.GetMetricWithLabelValues(language)
	if err != nil {
		logger.Error("Incrementing language counter for " + language + " failed: " + err.Error())
	} else {
		counter.Inc()
	}
}

// logProcessed logs throughput every numProcessed objects. Throughput is rounded for
// slightly prettier output.
func logProcessed() {
	numProcessed = numProcessed + 1
	if numProcessed == OBJECTS_PER_LOG {
		now := time.Since(startTime)
		throughput := fmt.Sprintf("%.2f", float64(OBJECTS_PER_LOG)/now.Seconds())
		logger.Info("Processed "+strconv.Itoa(OBJECTS_PER_LOG)+" objects in "+now.String()+" ("+throughput+" per second)", map[string]string{"took": now.String()}, map[string]string{"throughput": throughput})
		numProcessed = 0
		startTime = time.Now()
	}
}
