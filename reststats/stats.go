package reststats

import "time"

var CURIOSITY = 1000
var CURIOSITY_FAILED = 100
var CURIOSITY_SLOW = 100
var SLOW_MS = 100
var QUICK_SEQUENCE_SIZE = 100

type statsData struct {
	started                  time.Time
	requestTotal             int
	requestsByEndpoint       map[string]int
	responseStats            map[string]int
	currentRequestTime       time.Time
	previousRequestTime      time.Time
	history                  []*responseStatsData
	historyOfFailed          []*responseStatsData
	historyOfSlow            []*responseStatsData
	shortestSequenceDuration time.Duration
}

type responseStatsData struct {
	time       time.Time
	url        string
	statusCode int
	duration   time.Duration
}

var stats = &statsData{
	started:                  time.Now(),
	requestTotal:             0,
	requestsByEndpoint:       map[string]int{},
	responseStats:            getEmptyCountsByStatusCodeMap(),
	currentRequestTime:       time.Now(),
	previousRequestTime:      time.Now(),
	history:                  make([]*responseStatsData, 0, CURIOSITY),
	historyOfFailed:          make([]*responseStatsData, 0, CURIOSITY_FAILED),
	historyOfSlow:            make([]*responseStatsData, 0, CURIOSITY_SLOW),
	shortestSequenceDuration: -1,
}

func getStats() *statsData {
	return stats
}

func startHandlingStats() (chan<- int, chan<- string, chan<- *responseStatsData) {
	requests := make(chan int)
	endpoints := make(chan string)
	responseStats := make(chan *responseStatsData)

	go countRequests(requests)
	go countRequestsByEndpoint(endpoints)
	go updateResponseStats(responseStats)

	return requests, endpoints, responseStats
}

func countRequests(ch <-chan int) {
	for {
		n := <-ch
		stats.requestTotal += n
		stats.previousRequestTime = stats.currentRequestTime
		stats.currentRequestTime = time.Now()
	}
}

func countRequestsByEndpoint(ch <-chan string) {
	for {
		endpoint := <-ch
		val, ok := stats.requestsByEndpoint[endpoint]
		if !ok {
			val = 0
		}
		stats.requestsByEndpoint[endpoint] = val + 1
	}
}

func updateResponseStats(ch <-chan *responseStatsData) {
	for {
		responseStats := <-ch
		stats.history = shiftAndPush(stats.history, responseStats, CURIOSITY)
		if responseStats.statusCode >= 400 {
			stats.historyOfFailed = shiftAndPush(stats.historyOfFailed, responseStats, CURIOSITY_FAILED)
		}
		if responseStats.duration >= time.Duration(SLOW_MS)*time.Millisecond {
			stats.historyOfSlow = shiftAndPush(stats.historyOfSlow, responseStats, CURIOSITY_SLOW)
		}

		updateCountsByStatusCodeMap(stats.responseStats, responseStats.statusCode)

		if len(stats.history) >= QUICK_SEQUENCE_SIZE {
			lastSequenceDuration := stats.history[len(stats.history)-1].time.Sub(
				stats.history[len(stats.history)-QUICK_SEQUENCE_SIZE].time)
			if stats.shortestSequenceDuration == -1 || stats.shortestSequenceDuration > lastSequenceDuration {
				stats.shortestSequenceDuration = lastSequenceDuration
			}
		}
	}
}

func getEmptyCountsByStatusCodeMap() map[string]int {
	return map[string]int{
		"1XX": 0,
		"2XX": 0,
		"3XX": 0,
		"4XX": 0,
		"5XX": 0,
	}
}

func updateCountsByStatusCodeMap(responseMap map[string]int, statusCode int) {
	if statusCode >= 500 {
		responseMap["5XX"]++
	} else if statusCode >= 400 {
		responseMap["4XX"]++
	} else if statusCode >= 300 {
		responseMap["3XX"]++
	} else if statusCode >= 200 {
		responseMap["2XX"]++
	} else {
		responseMap["1XX"]++
	}
}

func shiftAndPush(slice []*responseStatsData, item *responseStatsData, maxLength int) []*responseStatsData {
	if len(slice) == maxLength {
		// TODO: study performance implications of this
		slice = slice[1:]
	}
	slice = append(slice, item)
	return slice
}
