package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/cpu"
)

func main() {
	mt, err := NewMetricTracker(100*time.Millisecond, time.Minute)
	if err != nil {
		log.Fatalf("initialize metric tracker: %v\n", err)
	}
	http.HandleFunc("/", mt.handleMetricRequest)
	port := ":8080"
	log.Printf("listening on %s...", port)
	log.Fatal(http.ListenAndServe(port, nil))
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
}

func NewMetricTracker(freq time.Duration, limit time.Duration) (*MetricTracker, error) {
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
	}
	go mt.Track()
	return mt, nil
}

func (m *MetricTracker) Track() {
	for {
		value, err := cpu.Percent(m.freq, false)
		if err != nil {
			log.Fatalf("%v", err)
		}
		m.data[m.head] = Value{
			Ts:  time.Now().UnixNano(),
			Val: value[0],
		}
		if (m.head + 1) == m.size {
			m.hasWrapped = true
		}
		m.head = (m.head + 1) % m.size
	}
}

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
