package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	irukaLogger "github.com/bottlenose-inc/go-common-tools/logger" // go-common-tools bunyan-style logger package
	"github.com/bottlenose-inc/go-common-tools/metrics"            // go-common-tools Prometheus metrics package
	"github.com/stretchr/testify/assert"                    // Assertion package
)

var (
	server    *httptest.Server
	serverUrl string
)

func TestMain(m *testing.M) {
	server = httptest.NewServer(getRouter())
	serverUrl = fmt.Sprintf("%s/", server.URL)
	logger, _ = irukaLogger.NewLogger(AUGMENTATION_NAME+"-test", "/dev/null") // Ignore log messages
	defer logger.Close()

	// Start Prometheus metrics server and initialize metrics to avoid panic during tests
	go metrics.StartPrometheusMetricsServer(AUGMENTATION_NAME+"-test", logger, PROMETHEUS_PORT)
    InitMetrics()

	// load data file
	langFile, err := ioutil.ReadFile(LANG_FILE)
	if err != nil {
		logger.Fatal("File error: " + err.Error())
		os.Exit(1)
	}
	err = json.Unmarshal(langFile, &KnownLanguages)
	if err != nil {
		logger.Fatal("File loading error: " + err.Error())
		os.Exit(1)
	}

	// Prepare responses
	GenerateResponses()

	// Run tests
	run := m.Run()
	os.Exit(run)
}

func TestUsage(t *testing.T) {
	fmt.Println(">> Testing GET / (usage information)...")

	// Perform request
	resp, err := http.Get(serverUrl)
	assert.Nil(t, err, "request should not error")
	defer resp.Body.Close()

	// Read response
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err, "should not error reading response")
	assert.Equal(t, 200, resp.StatusCode, "response status code should be 200")
	expected := `{"augmentationProtocolVersion":1.0,"result":{"id":"language-detector","name":"language-detector","description":"Determine language code from text","in":{"text":{"type":"string"}},"out":{"iso6391code":{"type":"string"},"name":{"type":"string"}}}}`

	assert.Equal(t, []byte(expected), body, "usage information should match")
}

func TestNotFound(t *testing.T) {
	fmt.Println(">> Testing GET /fourohfour (not found)...")

	// Perform request
	resp, err := http.Get(serverUrl + "fourohfour")
	assert.Nil(t, err, "request should not error")
	defer resp.Body.Close()

	// Read response
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err, "should not error reading response")
	assert.Equal(t, 404, resp.StatusCode, "response status code should be 404")
	expected := `{"error":"Not found"}`
	assert.Equal(t, []byte(expected), body, "not found response should match")
}

func TestBadJson(t *testing.T) {
	fmt.Println(">> Testing POST / (with bad JSON)...")

	// Prepare request
	reader := strings.NewReader(`{]}`)

	// Perform request
	resp, err := http.Post(serverUrl, "application/json", reader)
	assert.Nil(t, err, "request should not error")
	defer resp.Body.Close()

	// Read response
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err, "should not error reading response")
	assert.Equal(t, 400, resp.StatusCode, "response status code should be 400")
	expected := `{"error":"Unable to parse request - invalid JSON detected"}`
	assert.Equal(t, []byte(expected), body, "not found response should match")
}

func TestMissingTextKey(t *testing.T) {
	fmt.Println(">> Testing POST with input missing text key...")

	// prepare request
	reader := strings.NewReader(`{"request": [{"bad_text": "This is an invalid input test."}]}`)

	// perform request
	resp, err := http.Post(serverUrl, "application/json", reader)
	assert.Nil(t, err, "request should not error")
	defer resp.Body.Close()

	// read response
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err, "should not error reading response")
	assert.Equal(t, 400, resp.StatusCode, "response status code should be 200")
	expected := `{"response":[{"error":"Missing text key"}]}`
	assert.Equal(t, []byte(expected), body, "response should match")
}

func TestValidInput(t *testing.T) {
	fmt.Println(">> Testing POST with valid input...")

	// prepare request
	reader := strings.NewReader(`{"request": [{"text": "This is a valid input test."}]}`)

	// perform request
	resp, err := http.Post(serverUrl, "application/json", reader)
	assert.Nil(t, err, "request should not error")
	defer resp.Body.Close()

	// read response
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err, "should not error reading response")
	assert.Equal(t, 200, resp.StatusCode, "response status code should be 200")

	expected := `{"response":[{"iso6391code":"en","name":"English"}]}`
	assert.Equal(t, []byte(expected), body, "response should match")
}

func TestLanguageDetection(t *testing.T) {
	fmt.Println("Testing language detection accuracy")
	// Tests ported from node langugage-detector code

	testText := "para poner este importante proyecto en práctica"
	code := Detect_language(testText)
	assert.Equal(t, "es", code)
	name, found := KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Spanish", name)

	testText = "this is a test of the Emergency text categorizing system."
	code = Detect_language(testText)
	assert.Equal(t, "en", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "English", name)

	testText = "serait(désigné peu après PDG d'Antenne 2 et de FR 3. Pas même lui ! Le"
	code = Detect_language(testText)
	assert.Equal(t, "fr", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "French", name)

	testText = "studio dell'uomo interiore? La scienza del cuore umano, che"
	code = Detect_language(testText)
	assert.Equal(t, "it", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Italian", name)

	testText = "taiate pe din doua, in care vezi stralucind brun  sau violet cristalele interioare"
	code = Detect_language(testText)
	assert.Equal(t, "ro", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Romanian", name)

	testText = "na porozumieniu, na ³±czeniu si³ i ¶rodków. Dlatego szukam ludzi, którzy"
	code = Detect_language(testText)
	assert.Equal(t, "pl", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Polish", name)

	testText = "sagt Hühsam das war bei Über eine Annonce in einem Frankfurter der Töpfer ein. Anhand von gefundenen gut kennt, hatte ihm die wahren Tatsachen Sechzehn Adorno-Schüler erinnern und daß ein Weiterdenken der Theorie für ihre Festlegung sind drei Jahre Erschütterung Einblick in die Abhängigkeit(der Bauarbeiten sei"
	code = Detect_language(testText)
	assert.Equal(t, "de", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "German", name)

	testText = "esôzéseket egy kissé túlméretezte, ebbôl kifolyólag a Földet egy hatalmas árvíz mosta el"
	code = Detect_language(testText)
	assert.Equal(t, "hu", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Hungarian", name)

	testText = "koulun arkistoihin pölyttymään, vaan nuoret saavat itse vaikuttaa ajatustensa eteenpäinviemiseen esimerkiksi"
	code = Detect_language(testText)
	assert.Equal(t, "fi", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Finnish", name)

	testText = "tegen de kabinetsplannen. Een speciaal in het leven geroepen Landelijk"
	code = Detect_language(testText)
	assert.Equal(t, "nl", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Dutch", name)

	testText = "viksomhed, 58 pct. har et arbejde eller er under uddannelse, 76 pct. forsørges ikke længere af Kolding"
	code = Detect_language(testText)
	assert.Equal(t, "da", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Danish", name)

	testText = "datují rokem 1862.  Naprosto zakázán byl v pocitech smutku, beznadìje èi jiné"
	code = Detect_language(testText)
	assert.Equal(t, "cs", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Czech", name)

	testText = "hovedstaden Nanjings fall i desember ble byens innbyggere utsatt for et seks"
	code = Detect_language(testText)
	assert.Equal(t, "no", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Norwegian", name)

	testText = "popular. Segundo o seu biógrafo, a Maria Adelaide auxiliava muita gente"
	code = Detect_language(testText)
	assert.Equal(t, "pt", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Portuguese", name)

	testText = "TaffyDB finders looking nice so far! Testing this long sentence."
	code = Detect_language(testText)
	assert.Equal(t, "en", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "English", name)

	testText = "Och så ska vi prova lite svenska, som också borde fungera utan problem."
	code = Detect_language(testText)
	assert.Equal(t, "sv", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Swedish", name)
}

func TestLanguageDetectionMore(t *testing.T) {
	fmt.Println("Testing language detection accuracy for more languages")

	testText := " 私はガラスを食べられます。それは私を傷つけません。"
	code := Detect_language(testText)
	assert.Equal(t, "ja", code)
	name, found := KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Japanese", name)

	testText = "我能吞下玻璃而不伤身体。"
	code = Detect_language(testText)
	assert.Equal(t, "zh", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Chinese", name)

	testText = "나는 유리를 먹을 수 있어요. 그래도 아프지 않아요"
	code = Detect_language(testText)
	assert.Equal(t, "ko", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Korean", name)

	testText = "أنا قادر على أكل الزجاج و هذا لا يؤلمني. "
	code = Detect_language(testText)
	assert.Equal(t, "ar", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Arabic", name)

	testText = "ฉันกินกระจกได้ แต่มันไม่ทำให้ฉันเจ็บ"
	code = Detect_language(testText)
	assert.Equal(t, "th", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Thai", name)

	testText = ".من می توانم بدونِ احساس درد شيشه بخورم"
	code = Detect_language(testText)
	assert.Equal(t, "fa", code)
	name, found = KnownLanguages[code]
	assert.Equal(t, true, found)
	assert.Equal(t, "Persian", name)
}

func TestStripNames(t *testing.T) {
	fmt.Println(">> Testing input with twitter handles...")

	// prepare request
	reader := strings.NewReader(`{"request": [{"text": "RT @VictoriaMo02: @SoofyAcosta al fin me contesto él wpp jajaja te amo sofy"}]}`)

	// perform request
	resp, err := http.Post(serverUrl, "application/json", reader)
	assert.Nil(t, err, "request should not error")
	defer resp.Body.Close()

	// read response
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err, "should not error reading response")
	assert.Equal(t, 200, resp.StatusCode, "response status code should be 200")

	expected := `{"response":[{"iso6391code":"es","name":"Spanish"}]}`
	assert.Equal(t, []byte(expected), body, "response should match")
}

func TestStripLinks(t *testing.T) {
	fmt.Println(">> Testing input with links...")

	// prepare request
	reader := strings.NewReader(`{"request": [{"text": "Mengalami Turbulensi Dahsyat, 23 Penumpang Avianca Airbus Terluka https://t.co/6SvpzBOKHT https://t.co/qYzmaPv7Od"}]}`)

	// perform request
	resp, err := http.Post(serverUrl, "application/json", reader)
	assert.Nil(t, err, "request should not error")
	defer resp.Body.Close()

	// read response
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err, "should not error reading response")
	assert.Equal(t, 200, resp.StatusCode, "response status code should be 200")

	expected := `{"response":[{"iso6391code":"ms","name":"Malay"}]}`
	assert.Equal(t, []byte(expected), body, "response should match")
}
