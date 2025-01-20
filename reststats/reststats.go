package reststats

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var version string = ""
var requestChannel chan<- int
var endpointChannel chan<- string
var responseStatsChannel chan<- *responseStatsData

func Initialize(v string) {
	version = v

	requestChannel, endpointChannel, responseStatsChannel = startHandlingStats()
}

func CountRequestByEndpoint(endpoint string) {
	endpointChannel <- endpoint
}

func HandleEndpointWithStats(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		handler(c)
		duration := time.Since(start)

		endpointChannel <- c.Request.URL.Path

		responseStats := &responseStatsData{
			time:       start,
			url:        c.Request.RequestURI,
			statusCode: c.Writer.Status(),
			duration:   duration,
		}
		responseStatsChannel <- responseStats
	}
}

func HandleWithStats(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		handler(c)
		duration := time.Since(start)

		responseStats := &responseStatsData{
			time:       start,
			url:        c.Request.RequestURI,
			statusCode: c.Writer.Status(),
			duration:   duration,
		}
		responseStatsChannel <- responseStats
	}
}

func UpdateResponseStatsOnRecover(start time.Time, url string, statusCode int) {
	responseStats := &responseStatsData{
		time:       start,
		url:        url,
		statusCode: statusCode,
		duration:   0,
	}
	responseStatsChannel <- responseStats
}

func RequestCounter() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestChannel <- 1
	}
}

type statsResult struct {
	Version                             string                `json:"version"`
	Uptime                              string                `json:"uptime"`
	RequestsTotal                       int                   `json:"requests_total"`
	TimeSinceLastRequest                string                `json:"time_since_last_request"`
	RequestsByEndpoint                  map[string]int        `json:"requests_by_endpoint"`
	Last1000Requests                    *last1000RequestsData `json:"last_1000_requests"`
	ShortestInterval100RequestsReceived string                `json:"shortest_interval_100_requests_received"`
	ResponsesAll                        map[string]int        `json:"responses_all"`
	ResponsesLast1000                   map[string]int        `json:"responses_last_1000"`
	RequestsLast10                      []*requestStatsData   `json:"requests_last_10"`
	FailedRequestsLast10                []*requestStatsData   `json:"failed_requests_last_10"`
	SlowRequestsLast10                  []*requestStatsData   `json:"slow_requests_last_10"`
}

type requestStatsData struct {
	Url        string `json:"url"`
	StatusCode int    `json:"statusCode"`
	Duration   int64  `json:"duration"`
}

type last1000RequestsData struct {
	DoneWithin  string `json:"done_within"`
	MinDuration int64  `json:"min_duration"`
	MaxDuration int64  `json:"max_duration"`
	AvgDuration int64  `json:"avg_duration"`
}

func HandleGetStats(c *gin.Context) {
	stats = getStats()
	now := time.Now()

	responsesHistory := getResponseHistory(stats.history)
	requestsLast10 := getLast10Requests(stats.history)
	failedRequestsLast10 := getLast10Requests(stats.historyOfFailed)
	slowRequestsLast10 := getLast10Requests(stats.historyOfSlow)

	result := &statsResult{
		Version:                             version,
		Uptime:                              getTimeDiffFormatted(stats.started, now),
		RequestsTotal:                       stats.requestTotal,
		TimeSinceLastRequest:                getTimeDiffFormatted(stats.previousRequestTime, now),
		RequestsByEndpoint:                  stats.requestsByEndpoint,
		Last1000Requests:                    getLast1000RequestData(stats.history),
		ShortestInterval100RequestsReceived: getTimeIntervalFormatted(stats.shortestSequenceDuration),
		ResponsesAll:                        stats.responseStats,
		ResponsesLast1000:                   responsesHistory,
		RequestsLast10:                      requestsLast10,
		FailedRequestsLast10:                failedRequestsLast10,
		SlowRequestsLast10:                  slowRequestsLast10,
	}

	c.JSON(http.StatusOK, result)
}

func getTimeDiffFormatted(start time.Time, end time.Time) string {
	return getTimeIntervalFormatted(end.Sub(start))
}

func getTimeIntervalFormatted(duration time.Duration) string {
	SECONDS_IN_DAY := 86400.0
	SECONDS_IN_HOUR := 3600.0
	SECONDS_IN_MINUTES := 60.0

	diff := duration.Seconds()

	days := math.Floor(diff / SECONDS_IN_DAY)
	diff = diff - days*SECONDS_IN_DAY

	hours := math.Floor(diff / SECONDS_IN_HOUR)
	diff = diff - hours*SECONDS_IN_HOUR

	minutes := math.Floor(diff / SECONDS_IN_MINUTES)
	diff = diff - minutes*SECONDS_IN_MINUTES

	seconds := math.Floor(diff)

	return fmt.Sprintf("%d.%d:%d:%d", int(days), int(hours), int(minutes), int(seconds))
}

func getResponseHistory(history []*responseStatsData) map[string]int {
	responsesHistory := getEmptyCountsByStatusCodeMap()
	for _, v := range history {
		updateCountsByStatusCodeMap(responsesHistory, v.statusCode)
	}
	return responsesHistory
}

func getLast10Requests(history []*responseStatsData) []*requestStatsData {
	requestsLast10 := make([]*requestStatsData, 0, 10)
	if len(history) > 0 {
		idx := len(history) - 10
		if idx < 0 {
			idx = 0
		}
		for _, v := range history[idx:] {
			requestsLast10 = append(requestsLast10,
				&requestStatsData{
					Url:        v.url,
					StatusCode: v.statusCode,
					Duration:   v.duration.Milliseconds(),
				})
		}
	}
	return requestsLast10
}

func getLast1000RequestData(history []*responseStatsData) *last1000RequestsData {
	last1000RequestsWithin := time.Duration(0)
	var last1000RequestsMinDuration int64 = math.MaxInt64
	var last1000RequestsMaxDuration int64 = 0
	var last1000RequestsTotalDuration int64 = 0
	var last1000RequestsAvgDuration int64 = 0

	if len(history) > 0 {
		last1000RequestsWithin = time.Since(history[0].time)
		for _, v := range history {
			if v.duration.Milliseconds() < last1000RequestsMinDuration {
				last1000RequestsMinDuration = v.duration.Milliseconds()
			}
			if v.duration.Milliseconds() > last1000RequestsMaxDuration {
				last1000RequestsMaxDuration = v.duration.Milliseconds()
			}
			last1000RequestsTotalDuration += v.duration.Milliseconds()
		}
		last1000RequestsAvgDuration = last1000RequestsTotalDuration / int64(len(history))
	}

	return &last1000RequestsData{
		DoneWithin:  getTimeIntervalFormatted(last1000RequestsWithin),
		MinDuration: last1000RequestsMinDuration,
		MaxDuration: last1000RequestsMaxDuration,
		AvgDuration: last1000RequestsAvgDuration,
	}
}
