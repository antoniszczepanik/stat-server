package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/cpu"
)

func main() {
	var port = flag.Int("p", 2137, "Port to listen on")
	var freq = flag.Int("f", 100, "Frequency at which metrics will be stored, in milliseconds")
	var limit = flag.Int("l", 60, "How much history will be persisted, in seconds")
	flag.Parse()

	mt, err := NewMetricTracker(
		time.Duration(*freq)*time.Millisecond,
		time.Duration(*limit)*time.Second,
		GetCpuUsage,
	)
	if err != nil {
		log.Fatalf("initialize metric tracker: %v\n", err)
	}

	http.HandleFunc("/", mt.handleMetricRequest)
	on := fmt.Sprintf(":%d", *port)
	log.Printf("listening on %s...", on)
	log.Fatal(http.ListenAndServe(on, nil))
}

type Value struct {
	Ts  int64   `json:"ts"`
	Val float64 `json:"value"`
}

func (v Value) String() string {
	return fmt.Sprintf("%d %f", v.Ts, v.Val)
}

type MetricTracker struct {
	data       []Value
	hasWrapped bool
	head       int
	size       int
	freq       time.Duration

	// getValue gets actual metric data at given point in time.
	getValue func(time.Duration) (float64, error)
}

// GetCpuUsage is an example of getValue function. It needs to block
// for interval duration.
func GetCpuUsage(interval time.Duration) (float64, error) {
	values, err := cpu.Percent(interval, false)
	if err != nil {
		return 0, err
	}
	return values[0], nil
}

func NewMetricTracker(freq time.Duration, limit time.Duration,
	getValue func(time.Duration) (float64, error)) (*MetricTracker, error) {
	if freq > limit {
		return nil, errors.New("metric storage time limit needs to be larger than frequency of polling")
	}
	size := int(limit / freq)
	mt := &MetricTracker{
		data:       make([]Value, size),
		head:       0,
		hasWrapped: false,
		size:       size,
		freq:       freq,
		getValue:   getValue,
	}
	go mt.Track()
	return mt, nil
}

func (m *MetricTracker) Track() {
	for {
		value, err := m.getValue(m.freq)
		if err != nil {
			log.Fatalf("%v", err)
		}
		m.data[m.head] = Value{
			Ts:  time.Now().UnixNano(),
			Val: value,
		}
		if (m.head + 1) == m.size {
			m.hasWrapped = true
		}
		m.head = (m.head + 1) % m.size
	}
}

// GetLast gets values for the most recent duration time.
func (m *MetricTracker) GetLast(duration time.Duration) []Value {
	result := []Value{}
	if m.hasWrapped {
		result = append(result, m.data[m.head:]...)
	}
	result = append(result, m.data[:m.head]...)
	count := int(duration / m.freq)
	if count >= len(result) {
		return result
	}
	return result[len(result)-count:]
}

func (m *MetricTracker) handleMetricRequest(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	var reqDurationUnit time.Duration
	for k, v := range params {
		switch k {
		case "ns":
			reqDurationUnit = time.Nanosecond
		case "ms":
			reqDurationUnit = time.Millisecond
		case "s":
			reqDurationUnit = time.Second
		default:
			http.Error(w, fmt.Sprintf("bad request: unknown query param: %s", k), 400)
			return
		}
		if len(v) != 1 {
			http.Error(w, "bad request: invalid query param value", 400)
			return
		}
		count, err := strconv.ParseInt(v[0], 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("bad request: invalid query param value: %v", v[0]), 400)
			return
		}
		lastValues := m.GetLast(reqDurationUnit * time.Duration(count))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(lastValues)
		return
	}
	http.Error(w, "bad request: missing query param", 400)
	return
}
